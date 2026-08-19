package adapter

import "testing"

func TestCatalogIDsAreUniqueAndLabeled(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tool := range Catalog() {
		if tool.ID == "" || tool.Label == "" {
			t.Fatalf("empty catalog row %+v", tool)
		}
		if _, ok := seen[tool.ID]; ok {
			t.Fatalf("duplicate id %s", tool.ID)
		}
		seen[tool.ID] = struct{}{}
		if Label(tool.ID) != tool.Label {
			t.Fatalf("Label(%s)=%q", tool.ID, Label(tool.ID))
		}
	}
	if len(KnownIDs()) != len(Catalog()) {
		t.Fatal("KnownIDs drifted from Catalog")
	}
}
