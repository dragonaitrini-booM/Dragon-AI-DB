package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// processAndReport reads a CSV file at path, computes a tiny summary and writes a JSON report to outputDir.
func processAndReport(path string, outputDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// read header
	header, err := r.Read()
	if err == io.EOF {
		return fmt.Errorf("empty csv")
	}
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	colCount := len(header)
	rows := 0
	// try to sum numeric columns
	numericSums := make([]float64, colCount)
	numericCount := make([]int, colCount)

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// continue on malformed rows but count them
			continue
		}
		rows++
		for i, cell := range row {
			if cell == "" {
				continue
			}
			if v, err := strconv.ParseFloat(cell, 64); err == nil {
				numericSums[i] += v
				numericCount[i]++
			}
		}
	}

	report := map[string]interface{}{
		"source":       filepath.Base(path),
		"processed_at": time.Now().UTC().Format(time.RFC3339),
		"header":       header,
		"rows":         rows,
		"numeric_summary": func() map[string]map[string]interface{} {
			m := map[string]map[string]interface{}{}
			for i := 0; i < colCount; i++ {
				if numericCount[i] > 0 {
					m[header[i]] = map[string]interface{}{
						"sum":   numericSums[i],
						"count": numericCount[i],
						"avg":   numericSums[i] / float64(numericCount[i]),
					}
				}
			}
			return m
		}(),
	}

	outName := fmt.Sprintf("%s.report.json", filepath.Base(path))
	outPath := filepath.Join(outputDir, outName)
	outF, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	defer outF.Close()

	enc := json.NewEncoder(outF)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}
