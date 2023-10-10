package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DirectoryConfig struct {
	Path      string
	MaxSizeMB int64
}

type Config struct {
	Directories []DirectoryConfig
}

func main() {
	// Otwarcie pliku dziennika
	logFile, err := os.OpenFile("program.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Błąd otwarcia pliku log:", err)
		return
	}
	defer logFile.Close()

	// Inicjalizacja loggera
	logger := log.New(logFile, "", log.LstdFlags)
	logger.Println("Program uruchomiony:", time.Now())

	// Odczyt konfiguracji
	config, err := readConfig("settings.cfg")
	if err != nil {
		logger.Println("Błąd odczytu konfiguracji:", err)
		return
	}

	// Przetwarzanie katalogów z konfiguracji
	for _, dirConfig := range config.Directories {
		// Odczyt zawartości katalogu
		files, err := os.ReadDir(dirConfig.Path)
		if err != nil {
			logger.Println("Błąd odczytu katalogu:", err)
			continue
		}

		var totalSize int64

		// Obliczanie całkowitego rozmiaru plików w katalogu
		for _, file := range files {
			info, err := file.Info()
			if err != nil {
				logger.Println("Błąd pobierania informacji o pliku:", err)
				continue
			}
			totalSize += info.Size()
		}

		maxDirectorySizeBytes := dirConfig.MaxSizeMB * 1024 * 1024
		freedSpace := int64(0)

		// Usuwanie plików, aby zmniejszyć rozmiar katalogu
		for totalSize > maxDirectorySizeBytes {
			if len(files) == 0 {
				break
			}

			// Ustaw licznik prób usuwania na zero
			removeAttempts := 0

			for _, oldestFile := range files {
				// Sprawdź, czy liczba prób usunięcia jest mniejsza niż 5
				if removeAttempts >= 5 {
					logger.Printf("Przekroczono limit prób usuwania pliku %s. Pomijanie.", oldestFile.Name())
					break
				}

				filePath := filepath.Join(dirConfig.Path, oldestFile.Name())
				err := os.Remove(filePath)
				if err != nil {
					if os.IsPermission(err) {
						// Obsługa błędu "Odmowa dostępu"
						logger.Printf("Błąd usuwania pliku %s: %v", oldestFile.Name(), err)
						removeAttempts++
						continue // Pominięcie pliku w przypadku "Odmowy dostępu"
					} else {
						// Inne rodzaje błędów, które mogą wymagać innego postępowania
						logger.Printf("Inny błąd usuwania pliku %s: %v", oldestFile.Name(), err)
					}
				}

				info, err := oldestFile.Info()
				if err != nil {
					logger.Println("Błąd pobierania informacji o pliku:", err)
					continue
				}

				totalSize -= info.Size()
				freedSpace += info.Size()
				logger.Printf("Usunięto plik: %s\n", oldestFile.Name())

				if totalSize <= maxDirectorySizeBytes {
					break
				}
			}

			// Usuń plik z listy, aby uniknąć ponownego próbowania usunięcia
			files = removeProcessedFiles(files, removeAttempts)
		}

		logger.Printf("Zwolniono %d MB w katalogu %s\n", freedSpace/(1024*1024), dirConfig.Path)
	}
}

// Funkcja pomocnicza do usuwania przetworzonych plików
func removeProcessedFiles(files []fs.DirEntry, removeAttempts int) []fs.DirEntry {
	var remainingFiles []fs.DirEntry
	for i, file := range files {
		if i >= removeAttempts {
			remainingFiles = append(remainingFiles, file)
		}
	}
	return remainingFiles
}

func readConfig(filename string) (Config, error) {
	var config Config

	file, err := os.Open(filename)
	if err != nil {
		return config, err
	}
	defer file.Close()

	var currentDirectory DirectoryConfig

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err == io.EOF {
			break
		}

		line := scanner.Text()
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "directory_path":
				currentDirectory.Path = value
			case "max_directory_size":
				sizeMB, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return config, err
				}
				currentDirectory.MaxSizeMB = sizeMB
				config.Directories = append(config.Directories, currentDirectory)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return config, err
	}

	return config, nil
}
