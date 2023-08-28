package main

import (
	"bufio"
	"fmt"
	"io"
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
	logFile, err := os.OpenFile("program.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Błąd otwarcia pliku log:", err)
		return
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags)
	logger.Println("Program uruchomiony:", time.Now())

	config, err := readConfig("settings.cfg")
	if err != nil {
		logger.Println("Błąd odczytu konfiguracji:", err)
		return
	}

	for _, dirConfig := range config.Directories {
		files, err := os.ReadDir(dirConfig.Path)
		if err != nil {
			logger.Println("Błąd odczytu katalogu:", err)
			continue
		}

		var totalSize int64
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

		for totalSize > maxDirectorySizeBytes {
			if len(files) == 0 {
				break
			}

			for i := 0; i < len(files); i++ {
				oldestFile := files[i]
				oldestFileIndex := i

				for j := i + 1; j < len(files); j++ {
					infoI, err := oldestFile.Info()
					infoJ, err := files[j].Info()
					if err != nil {
						logger.Println("Błąd pobierania informacji o pliku:", err)
						continue
					}
					if infoJ.ModTime().Before(infoI.ModTime()) {
						oldestFile = files[j]
						oldestFileIndex = j
					}
				}

				filePath := filepath.Join(dirConfig.Path, oldestFile.Name())
				err := os.Remove(filePath)
				if err != nil {
					logger.Println("Błąd usuwania pliku:", err)
					return
				}

				info, err := oldestFile.Info()
				if err != nil {
					logger.Println("Błąd pobierania informacji o pliku:", err)
					continue
				}

				totalSize -= info.Size()
				freedSpace += info.Size()
				files = append(files[:oldestFileIndex], files[oldestFileIndex+1:]...)
				logger.Printf("Usunięto plik: %s\n", oldestFile.Name())

				if totalSize <= maxDirectorySizeBytes {
					break
				}
			}
		}

		logger.Printf("Zwolniono %d MB w katalogu %s\n", freedSpace/(1024*1024), dirConfig.Path)
	}
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
