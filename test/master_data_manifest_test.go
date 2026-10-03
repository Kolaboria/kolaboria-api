package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type catalogManifest struct {
	Table string `json:"table"`
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"items"`
}

type masterManifest struct {
	UUIDNamespace string            `json:"uuid_namespace"`
	Catalogs      []catalogManifest `json:"catalogs"`
}

func readMasterManifest(t *testing.T) masterManifest {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", "master_data.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest masterManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestSeedManifestHasStableUUIDsAndMatchesMigrationStatements(t *testing.T) {
	manifest := readMasterManifest(t)
	namespace, err := uuid.Parse(manifest.UUIDNamespace)
	if err != nil {
		t.Fatal(err)
	}
	seedUp, err := os.ReadFile(filepath.Join("..", "migrations", "000002_seed_master_data.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	seedDown, err := os.ReadFile(filepath.Join("..", "migrations", "000002_seed_master_data.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string]int{"contribution_roles": 30, "skills": 50, "tools": 80}
	slugPattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	total := 0
	for _, catalog := range manifest.Catalogs {
		if len(catalog.Items) != wantCounts[catalog.Table] {
			t.Fatalf("%s count = %d", catalog.Table, len(catalog.Items))
		}
		seenSlug := make(map[string]struct{}, len(catalog.Items))
		seenName := make(map[string]struct{}, len(catalog.Items))
		for _, item := range catalog.Items {
			wantID := uuid.NewSHA1(namespace, []byte(catalog.Table+":"+item.Slug))
			if item.ID != wantID.String() {
				t.Errorf("%s/%s id = %s, want %s", catalog.Table, item.Slug, item.ID, wantID)
			}
			if strings.TrimSpace(item.Name) != item.Name || item.Name == "" || !slugPattern.MatchString(item.Slug) {
				t.Errorf("invalid seed item: %+v", item)
			}
			if _, ok := seenSlug[item.Slug]; ok {
				t.Errorf("duplicate slug %s/%s", catalog.Table, item.Slug)
			}
			seenSlug[item.Slug] = struct{}{}
			nameKey := strings.ToLower(item.Name)
			if _, ok := seenName[nameKey]; ok {
				t.Errorf("duplicate name %s/%s", catalog.Table, item.Name)
			}
			seenName[nameKey] = struct{}{}
			upStatement := "('" + item.ID + "', '" + strings.ReplaceAll(item.Name, "'", "''") + "', '" + item.Slug + "')"
			downStatement := "('" + item.ID + "', '" + item.Slug + "')"
			if !strings.Contains(string(seedUp), upStatement) || !strings.Contains(string(seedDown), downStatement) {
				t.Errorf("migration statements missing manifest item %s/%s", catalog.Table, item.Slug)
			}
			total++
		}
	}
	if len(manifest.Catalogs) != len(wantCounts) || total != 160 {
		t.Fatalf("catalog count/total = %d/%d, want %d/160", len(manifest.Catalogs), total, len(wantCounts))
	}
}
