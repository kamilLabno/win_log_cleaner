package main

import (
	"bufio"
	"fmt"
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
	Recursive bool
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
		err := processDirectory(dirConfig, logger)
		if err != nil {
			logger.Printf("Błąd przetwarzania katalogu %s: %v", dirConfig.Path, err)
		}
	}
}

func processDirectory(dirConfig DirectoryConfig, logger *log.Logger) error {
	return processDirectoryRecursively(dirConfig.Path, dirConfig.MaxSizeMB, dirConfig.Recursive, logger)
}

func processDirectoryRecursively(path string, maxSizeMB int64, recursive bool, logger *log.Logger) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
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

	maxDirectorySizeBytes := maxSizeMB * 1024 * 1024
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

			filePath := filepath.Join(path, oldestFile.Name())
			err := os.Remove(filePath)
			if err != nil {
				logger.Println("Błąd usuwania pliku:", err)
				return err
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

	logger.Printf("Zwolniono %d MB w katalogu %s\n", freedSpace/(1024*1024), path)
	for _, file := range files {
		if file.IsDir() {
			subDirPath := filepath.Join(path, file.Name())
			err := processDirectoryRecursively(subDirPath, maxSizeMB, recursive, logger)
			if err != nil {
				logger.Printf("Błąd przetwarzania podkatalogu %s: %v", subDirPath, err)
			}
		}
	}

	return nil
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
			case "recursive":
				recursive, err := strconv.ParseBool(value)
				if err != nil {
					return config, err
				}
				currentDirectory.Recursive = recursive
				config.Directories = append(config.Directories, currentDirectory)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return config, err
	}

	return config, nil
}
