// utils.go corrections for int64 types and file path validation

package utils

import "os"

// Function to validate file path
func ValidateFilePath(path string) error {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return fmt.Errorf("file does not exist: %s", path)
    }
    return nil
}