//go:build legacy_locations

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/syahrulfahmi/kolaboria-api/config"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/database"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
)

func main() {
	start := time.Now()

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// 2. Connect to database
	db, err := database.NewPostgresConnection(cfg.Database, false)
	if err != nil {
		log.Fatalf("failed to initialize database connection: %v", err)
	}

	log.Println("Database connection established. Starting location seeding...")

	// 3. Open CSV file
	csvFile, err := os.Open(`c:\Users\Fahmi\Downloads\wilayah_indonesia.csv`)
	if err != nil {
		log.Fatalf("failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read CSV file: %v", err)
	}

	if len(records) <= 1 {
		log.Fatalf("CSV file is empty or only contains headers")
	}

	log.Printf("Read %d rows from CSV. Processing data structure...", len(records)-1)

	// Maps for uniqueness and parent matching
	uniqueProvinces := make(map[string]*master.Province)
	uniqueRegencies := make(map[string]*master.Regency)
	uniqueDistricts := make(map[string]*master.District)

	// Slice of provinces, regencies, and districts for batch insertion
	var provinces []*master.Province
	var regencies []*master.Regency
	var districts []*master.District
	var villages []*master.Village

	// Temporary structure to hold rows
	type csvRow struct {
		province string
		regency  string
		district string
		village  string
	}
	rows := make([]csvRow, 0, len(records)-1)

	// Step 1: Read all unique provinces, regencies, and districts
	provIdx := 1
	regIdx := 1
	dstIdx := 1

	for idx, record := range records {
		if idx == 0 {
			continue // Skip header
		}
		row := csvRow{
			province: strings.TrimSpace(record[0]),
			regency:  strings.TrimSpace(record[1]),
			district: strings.TrimSpace(record[2]),
			village:  strings.TrimSpace(record[3]),
		}
		rows = append(rows, row)

		// Province
		if _, ok := uniqueProvinces[row.province]; !ok {
			prov := &master.Province{
				Name:      row.province,
				Code:      fmt.Sprintf("prv-%d", provIdx),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			uniqueProvinces[row.province] = prov
			provinces = append(provinces, prov)
			provIdx++
		}

		// Regency key: ProvinceName + ":" + RegencyName
		regKey := row.province + ":" + row.regency
		if _, ok := uniqueRegencies[regKey]; !ok {
			regType := "regency"
			if strings.Contains(strings.ToLower(row.regency), "kota") {
				regType = "city"
			}
			reg := &master.Regency{
				Name:      row.regency,
				Code:      fmt.Sprintf("reg-%d", regIdx),
				Type:      regType,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			uniqueRegencies[regKey] = reg
			regencies = append(regencies, reg)
			regIdx++
		}

		// District key: ProvinceName + ":" + RegencyName + ":" + DistrictName
		dstKey := regKey + ":" + row.district
		if _, ok := uniqueDistricts[dstKey]; !ok {
			dst := &master.District{
				Name:      row.district,
				Code:      fmt.Sprintf("dst-%d", dstIdx),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			uniqueDistricts[dstKey] = dst
			districts = append(districts, dst)
			dstIdx++
		}
	}

	// Step 2: Insert Provinces
	log.Printf("Inserting %d unique provinces...", len(provinces))
	if err := db.Create(&provinces).Error; err != nil {
		log.Fatalf("failed to insert provinces: %v", err)
	}

	// Create province lookup map
	provLookup := make(map[string]int64)
	for _, p := range provinces {
		provLookup[p.Name] = p.ID
	}

	// Step 3: Link and Insert Regencies
	for key, reg := range uniqueRegencies {
		parts := strings.Split(key, ":")
		provName := parts[0]
		reg.ProvinceID = provLookup[provName]
	}
	log.Printf("Inserting %d unique regencies...", len(regencies))
	if err := db.CreateInBatches(&regencies, 500).Error; err != nil {
		log.Fatalf("failed to insert regencies: %v", err)
	}

	// Create regency lookup map
	regLookup := make(map[string]int64)
	for key, reg := range uniqueRegencies {
		regLookup[key] = reg.ID
	}

	// Step 4: Link and Insert Districts
	for key, dst := range uniqueDistricts {
		parts := strings.Split(key, ":")
		regKey := parts[0] + ":" + parts[1]
		dst.RegencyID = regLookup[regKey]
	}
	log.Printf("Inserting %d unique districts...", len(districts))
	if err := db.CreateInBatches(&districts, 1000).Error; err != nil {
		log.Fatalf("failed to insert districts: %v", err)
	}

	// Create district lookup map
	dstLookup := make(map[string]int64)
	for key, dst := range uniqueDistricts {
		dstLookup[key] = dst.ID
	}

	// Step 5: Link and Batch Insert Villages
	log.Println("Preparing village data...")
	for idx, row := range rows {
		dstKey := row.province + ":" + row.regency + ":" + row.district
		districtID := dstLookup[dstKey]

		village := &master.Village{
			DistrictID: districtID,
			Name:       row.village,
			Code:       fmt.Sprintf("vil-%d", idx+1),
			PostalCode: "", // default empty
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		villages = append(villages, village)
	}

	log.Printf("Inserting %d villages in batches of 2000...", len(villages))
	if err := db.CreateInBatches(&villages, 2000).Error; err != nil {
		log.Fatalf("failed to insert villages: %v", err)
	}

	log.Printf("Location seeding completed successfully in %v!", time.Since(start))
}
