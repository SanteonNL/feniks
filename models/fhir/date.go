package fhir

import (
    "fmt"
    "time"
    "encoding/json"
    "strings"
)

// Date represents a FHIR date
type Date struct {
    time.Time
}

// NewDate creates a new Date from a time.Time
func NewDate(t time.Time) Date {
    return Date{Time: t}
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (d *Date) UnmarshalJSON(data []byte) error {
    // Remove quotes
    s := strings.Trim(string(data), `"`)
    if s == "" {
        return nil
    }

    // Try parsing in different formats
    formats := []string{
        "2006-01-02",  // YYYY-MM-DD
        "2006-01",     // YYYY-MM
        "2006",        // YYYY
    }

    var err error
    for _, format := range formats {
        d.Time, err = time.Parse(format, s)
        if err == nil {
            return nil
        }
    }

    return fmt.Errorf("invalid date format: %s", s)
}

// MarshalJSON implements the json.Marshaler interface
func (d Date) MarshalJSON() ([]byte, error) {
    if d.Time.IsZero() {
        return json.Marshal("")
    }
    return json.Marshal(d.Format("2006-01-02"))
}

// String returns the date in YYYY-MM-DD format
func (d Date) String() string {
    return d.Format("2006-01-02")
}
