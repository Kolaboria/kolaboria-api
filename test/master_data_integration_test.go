package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/config"
	httpapi "github.com/syahrulfahmi/kolaboria-api/internal/http"
	"github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/health"
	"github.com/syahrulfahmi/kolaboria-api/internal/modules/master"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMasterDataRoutesAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KOLABORIA_TEST_DATABASE_URL to a disposable database with migrations applied")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open test PostgreSQL database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		t.Fatal("ping test PostgreSQL database:", err)
	}

	handler := master.NewMasterHandler(master.NewMasterService(master.NewMasterRepository(db)))
	router := httpapi.NewRouter(&config.Config{App: config.AppConfig{Env: "test"}}, health.NewHealthHandler(), jwt.NewJWTService(config.JWTConfig{}), handler, nil)
	server := httptest.NewServer(router)
	defer server.Close()
	manifest := readMasterManifest(t)
	wantTotals := map[string]int64{"contribution_roles": 30, "skills": 50, "tools": 80}
	for _, catalog := range manifest.Catalogs {
		route := map[string]string{"contribution_roles": "contribution-roles", "skills": "skills", "tools": "tools"}[catalog.Table]
		type responseItem struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		var payload struct {
			Status int            `json:"status"`
			Data   []responseItem `json:"data"`
			Meta   struct {
				Page       int   `json:"page"`
				Limit      int   `json:"limit"`
				Total      int64 `json:"total"`
				TotalPages int64 `json:"total_pages"`
			} `json:"meta"`
		}
		response := getMasterResponse(t, server.Client(), fmt.Sprintf("%s/api/v1/master-data/%s", server.URL, route))
		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			t.Fatalf("%s status=%d body=%s", route, response.StatusCode, body)
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		if payload.Status != http.StatusOK || payload.Meta.Total != wantTotals[catalog.Table] || payload.Meta.Page != 1 || payload.Meta.Limit != 50 || len(payload.Data) != min(50, len(catalog.Items)) {
			t.Fatalf("%s response metadata/data mismatch: %+v", route, payload.Meta)
		}
		wantItems := make([]struct{ ID, Name, Slug string }, 0, len(catalog.Items))
		for _, item := range catalog.Items {
			wantItems = append(wantItems, struct{ ID, Name, Slug string }{ID: item.ID, Name: item.Name, Slug: item.Slug})
		}
		sort.Slice(wantItems, func(i, j int) bool {
			left, right := strings.ToLower(wantItems[i].Name), strings.ToLower(wantItems[j].Name)
			if left == right {
				return wantItems[i].ID < wantItems[j].ID
			}
			return left < right
		})
		compareItems := func(page int, actual []responseItem) {
			t.Helper()
			start := (page - 1) * 50
			end := min(start+50, len(wantItems))
			if start > len(wantItems) {
				start = len(wantItems)
			}
			if len(actual) != end-start {
				t.Fatalf("%s page %d length=%d, want %d", route, page, len(actual), end-start)
			}
			for index, got := range actual {
				want := wantItems[start+index]
				if got.ID != want.ID || got.Name != want.Name || got.Slug != want.Slug {
					t.Errorf("%s page %d item %d=%+v, want %+v", route, page, index, got, want)
				}
			}
		}
		compareItems(1, payload.Data)
		for page := 2; page <= 2; page++ {
			pageResponse := getMasterResponse(t, server.Client(), fmt.Sprintf("%s/api/v1/master-data/%s?page=%d&limit=50", server.URL, route, page))
			var next struct {
				Data []responseItem `json:"data"`
				Meta struct {
					Total      int64 `json:"total"`
					TotalPages int64 `json:"total_pages"`
				} `json:"meta"`
			}
			if err := json.NewDecoder(pageResponse.Body).Decode(&next); err != nil {
				pageResponse.Body.Close()
				t.Fatal(err)
			}
			pageResponse.Body.Close()
			if pageResponse.StatusCode != http.StatusOK || next.Meta.Total != wantTotals[catalog.Table] || next.Meta.TotalPages != 2 && len(wantItems) > 50 {
				t.Fatalf("%s page %d status/meta mismatch: %d %+v", route, page, pageResponse.StatusCode, next.Meta)
			}
			compareItems(page, next.Data)
		}
	}
	for _, query := range []string{"  rEaCt  ", "next-js", "%", "_", `\`} {
		response := getMasterResponse(t, server.Client(), server.URL+"/api/v1/master-data/tools?q="+url.QueryEscape(query))
		var result struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		needle := strings.ToLower(strings.TrimSpace(query))
		want := make([]struct{ id, name string }, 0)
		for _, item := range manifest.Catalogs[2].Items {
			if strings.Contains(strings.ToLower(item.Name), needle) || strings.Contains(strings.ToLower(item.Slug), needle) {
				want = append(want, struct{ id, name string }{item.ID, item.Name})
			}
		}
		sort.Slice(want, func(i, j int) bool { return strings.ToLower(want[i].name) < strings.ToLower(want[j].name) })
		if response.StatusCode != http.StatusOK || len(result.Data) != len(want) {
			t.Errorf("search q=%q status=%d matches=%d, want %d", query, response.StatusCode, len(result.Data), len(want))
			continue
		}
		for index, item := range result.Data {
			if item.ID != want[index].id {
				t.Errorf("search q=%q item %d id=%s, want %s", query, index, item.ID, want[index].id)
			}
		}
	}
	empty := getMasterResponse(t, server.Client(), server.URL+"/api/v1/master-data/tools?page=99&q=%25")
	var emptyResult struct {
		Data []json.RawMessage `json:"data"`
		Meta struct {
			Total      int64 `json:"total"`
			TotalPages int64 `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(empty.Body).Decode(&emptyResult); err != nil {
		empty.Body.Close()
		t.Fatal(err)
	}
	empty.Body.Close()
	if empty.StatusCode != http.StatusOK || emptyResult.Data == nil || len(emptyResult.Data) != 0 || emptyResult.Meta.Total != 0 || emptyResult.Meta.TotalPages != 0 {
		t.Fatalf("empty page response = status %d data=%v meta=%+v", empty.StatusCode, emptyResult.Data, emptyResult.Meta)
	}
}

func TestMasterDataSchemaHasOnlyFiveValidatedColumns(t *testing.T) {
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KOLABORIA_TEST_DATABASE_URL to a disposable database with migrations applied")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open test PostgreSQL database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	wantColumns := map[string]string{"id": "uuid", "name": "text", "slug": "text", "created_at": "timestamptz", "updated_at": "timestamptz"}
	for table := range map[string]struct{}{"contribution_roles": {}, "skills": {}, "tools": {}} {
		rows, err := sqlDB.Query("SELECT column_name, udt_name, is_nullable, COALESCE(column_default, '') FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1 ORDER BY ordinal_position", table)
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
		for rows.Next() {
			var column, dataType, nullable, defaultValue string
			if err := rows.Scan(&column, &dataType, &nullable, &defaultValue); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			seen[column] = true
			if dataType != wantColumns[column] || nullable != "NO" {
				t.Errorf("%s.%s type/nullability = %s/%s", table, column, dataType, nullable)
			}
			if column == "id" && !strings.Contains(defaultValue, "gen_random_uuid") {
				t.Errorf("%s.id default = %q", table, defaultValue)
			}
			if (column == "created_at" || column == "updated_at") && !strings.Contains(defaultValue, "now()") {
				t.Errorf("%s.%s default = %q", table, column, defaultValue)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if len(seen) != 5 {
			t.Errorf("%s has columns %v, want exactly five", table, seen)
		}
		var indexDefinitions []string
		if err := db.Raw("SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ?", table).Scan(&indexDefinitions).Error; err != nil {
			t.Fatal(err)
		}
		lowerNameIndex := false
		for _, definition := range indexDefinitions {
			if strings.Contains(strings.ToLower(definition), "unique") && strings.Contains(strings.ToLower(definition), "lower(name)") {
				lowerNameIndex = true
			}
		}
		if !lowerNameIndex {
			t.Errorf("%s is missing the unique lower(name) index", table)
		}
	}
	invalidRows := []struct {
		table string
		name  string
		slug  string
	}{
		{"skills", "   ", "whitespace-only"},
		{"skills", "Invalid slug", "Upper_Case"},
		{"contribution_roles", " Frontend Developer", "leading-space"},
		{"tools", "Duplicate case-folded name", "unique-test-name"},
		{"tools", "Different label", "javascript"},
	}
	invalidRows[3].name = "javascript"
	invalidRows[3].slug = "unique-test-name"
	for _, invalid := range invalidRows {
		if err := db.Exec("INSERT INTO "+invalid.table+" (name, slug) VALUES (?, ?)", invalid.name, invalid.slug).Error; err == nil {
			t.Errorf("%s accepted invalid or duplicate (%q, %q)", invalid.table, invalid.name, invalid.slug)
		}
	}
}

func getMasterResponse(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestSeedRollbackLifecycle(t *testing.T) {
	scenario := os.Getenv("KOLABORIA_TEST_SCENARIO")
	if scenario != "prepare-extra" && scenario != "verify-extra" && scenario != "cleanup-extra" {
		t.Skip("set KOLABORIA_TEST_SCENARIO for the disposable migration verifier")
	}
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("KOLABORIA_TEST_DATABASE_URL is required for this scenario")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open verifier database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if scenario == "prepare-extra" {
		item := map[string]any{"id": uuid.New(), "name": "Verifier Extra Tool", "slug": "verifier-extra-tool"}
		if err := db.Table("tools").Create(&item).Error; err != nil {
			t.Fatal("insert verifier-only catalog row:", err)
		}
		return
	}
	var roleCount, skillCount, toolCount int64
	for table, count := range map[string]*int64{"contribution_roles": &roleCount, "skills": &skillCount, "tools": &toolCount} {
		if err := db.Table(table).Count(count).Error; err != nil {
			t.Fatal(err)
		}
	}
	switch scenario {
	case "verify-extra":
		if roleCount != 0 || skillCount != 0 || toolCount != 1 {
			t.Fatalf("seed rollback counts roles=%d skills=%d tools=%d; want 0/0/1", roleCount, skillCount, toolCount)
		}
	case "cleanup-extra":
		if err := db.Table("tools").Where("slug = ?", "verifier-extra-tool").Delete(map[string]any{}).Error; err != nil {
			t.Fatal("remove verifier-only row:", err)
		}
		if toolCount != 81 {
			t.Fatalf("tools count before cleanup = %d, want 81", toolCount)
		}
	}
}

func TestSchemaDownPreservesUnrelatedTableAndReplays(t *testing.T) {
	scenario := os.Getenv("KOLABORIA_TEST_SCHEMA_SCENARIO")
	if scenario != "prepare" && scenario != "verify" && scenario != "cleanup" {
		t.Skip("set KOLABORIA_TEST_SCHEMA_SCENARIO for the disposable migration verifier")
	}
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("KOLABORIA_TEST_DATABASE_URL is required for this scenario")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open verifier database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	switch scenario {
	case "prepare":
		if err := db.Exec("CREATE TABLE verifier_unrelated_marker (id INTEGER PRIMARY KEY)").Error; err != nil {
			t.Fatal("create marker table:", err)
		}
		if err := db.Exec("INSERT INTO verifier_unrelated_marker (id) VALUES (1)").Error; err != nil {
			t.Fatal("insert marker:", err)
		}
	case "verify":
		var markerCount int64
		if err := db.Table("verifier_unrelated_marker").Count(&markerCount).Error; err != nil {
			t.Fatal("marker table was removed by schema down:", err)
		}
		if markerCount != 1 {
			t.Fatalf("marker count=%d, want 1", markerCount)
		}
		for _, table := range []string{"contribution_roles", "skills", "tools"} {
			if db.Migrator().HasTable(table) {
				t.Errorf("master table %s still exists after schema down", table)
			}
		}
	case "cleanup":
		if !db.Migrator().HasTable("tools") || !db.Migrator().HasTable("skills") || !db.Migrator().HasTable("contribution_roles") {
			t.Fatal("master tables were not recreated after migration replay")
		}
		var count int64
		for _, table := range []string{"contribution_roles", "skills", "tools"} {
			if err := db.Table(table).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			want := map[string]int64{"contribution_roles": 30, "skills": 50, "tools": 80}[table]
			if count != want {
				t.Errorf("%s count=%d, want %d after replay", table, count, want)
			}
		}
		if err := db.Exec("DROP TABLE verifier_unrelated_marker").Error; err != nil {
			t.Fatal("drop marker table:", err)
		}
	}
}

func TestSchemaDownForeignKeyGuard(t *testing.T) {
	scenario := os.Getenv("KOLABORIA_TEST_FK_SCENARIO")
	if scenario != "prepare" && scenario != "verify" {
		t.Skip("set KOLABORIA_TEST_FK_SCENARIO for the disposable migration verifier")
	}
	dsn := os.Getenv("KOLABORIA_TEST_FK_DATABASE_URL")
	if dsn == "" {
		t.Fatal("KOLABORIA_TEST_FK_DATABASE_URL is required for this scenario")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open FK verifier database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if scenario == "prepare" {
		itemID := uuid.New()
		if err := db.Table("tools").Create(&map[string]any{"id": itemID, "name": "FK Guard Tool", "slug": "fk-guard-tool"}).Error; err != nil {
			t.Fatal("insert FK guard row:", err)
		}
		if err := db.Exec("CREATE TABLE verifier_tool_references (tool_id UUID NOT NULL REFERENCES tools(id))").Error; err != nil {
			t.Fatal("create FK guard table:", err)
		}
		if err := db.Exec("INSERT INTO verifier_tool_references (tool_id) VALUES (?)", itemID).Error; err != nil {
			t.Fatal("insert FK guard reference:", err)
		}
		return
	}
	var itemCount, referenceCount int64
	if err := db.Table("tools").Where("slug = ?", "fk-guard-tool").Count(&itemCount).Error; err != nil {
		t.Fatal("tools table was not preserved after rejected schema down:", err)
	}
	if err := db.Table("verifier_tool_references").Count(&referenceCount).Error; err != nil {
		t.Fatal("consumer table was not preserved after rejected schema down:", err)
	}
	if itemCount != 1 || referenceCount != 1 {
		t.Fatalf("preserved rows tool/reference = %d/%d, want 1/1", itemCount, referenceCount)
	}
}

func TestMasterRepositoryCountAndFetchUseOneSnapshot(t *testing.T) {
	dsn := os.Getenv("KOLABORIA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KOLABORIA_TEST_DATABASE_URL to a disposable database with migrations applied")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("open test PostgreSQL database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	itemID := uuid.New()
	defer sqlDB.Exec("DELETE FROM tools WHERE id = $1", itemID)
	inserted := false
	callbackName := "test:insert-between-master-count-and-fetch"
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if inserted || tx.Statement.Table != "tools" || !strings.Contains(strings.ToLower(tx.Statement.SQL.String()), "count(") {
			return
		}
		if err := sqlDB.QueryRowContext(tx.Statement.Context, "INSERT INTO tools (id, name, slug) VALUES ($1, $2, $3) RETURNING id", itemID, "Snapshot verifier item", "snapshot-verifier-item").Scan(&itemID); err != nil {
			tx.AddError(err)
			return
		}
		inserted = true
	}); err != nil {
		t.Fatal("register test query callback:", err)
	}
	defer db.Callback().Query().Remove(callbackName)
	items, total, err := master.NewMasterRepository(db).List(t.Context(), master.CatalogTools, master.ListFilter{Limit: 100})
	if err != nil {
		t.Fatal("list tools in snapshot:", err)
	}
	if !inserted || total != 80 || len(items) != 80 {
		t.Fatalf("inserted=%t count=%d items=%d; count and fetch should share the original snapshot", inserted, total, len(items))
	}
}
