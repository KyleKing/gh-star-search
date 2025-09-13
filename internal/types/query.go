package types

// Schema represents the database schema for query generation
type Schema struct {
	Tables map[string]Table `json:"tables"`
}

// Table represents a database table schema
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Indexes []Index  `json:"indexes"`
}

// Column represents a database column
type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Searchable  bool   `json:"searchable"`
}

// Index represents a database index
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Type    string   `json:"type"` // btree, fts, etc.
}
