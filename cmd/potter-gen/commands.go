package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"github.com/akriventsev/potter/framework/codegen"
)

const defaultPotterImportPath = "github.com/akriventsev/potter"

func runInit() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	modulePath := fs.String("module", "", "Go module path")
	outputDir := fs.String("output", ".", "Output directory")
	potterImportPath := fs.String("potter-import-path", defaultPotterImportPath, "Potter framework import path")

	fs.Parse(os.Args[2:])

	if *protoPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --proto is required\n")
		os.Exit(1)
	}

	if *modulePath == "" {
		fmt.Fprintf(os.Stderr, "Error: --module is required\n")
		os.Exit(1)
	}

	if err := validateProtoFile(*protoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ensureOutputDir(*outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Парсинг proto файла
	spec, err := parseProtoFile(*protoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto file: %v\n", err)
		os.Exit(1)
	}

	// Если module path не указан, используем из spec или из go_package
	if *modulePath == "" {
		if spec.ModuleName != "" {
			*modulePath = spec.ModuleName
		} else {
			fmt.Fprintf(os.Stderr, "Error: --module is required or set module_name in potter.service option\n")
			os.Exit(1)
		}
	}

	config := &codegen.GeneratorConfig{
		ModulePath:      *modulePath,
		OutputDir:       *outputDir,
		PackageName:     filepath.Base(*modulePath),
		Overwrite:       true,
		PreserveUserCode: false,
		PotterImportPath: *potterImportPath, // Импорт из main ветки
	}

	// Запуск генераторов
	generators := []codegen.Generator{
		codegen.NewDomainGenerator(*outputDir),
		codegen.NewApplicationGenerator(*outputDir),
		codegen.NewInfrastructureGenerator(*outputDir),
		codegen.NewPresentationGenerator(*outputDir),
		codegen.NewMainGenerator(*outputDir),
	}

		for _, gen := range generators {
			if err := gen.Generate(spec, config); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", gen.Name(), err)
				os.Exit(1)
			}
		}

	// Автоматическая инициализация Go модулей
	var modulesInitialized bool
	if err := initializeGoModules(*outputDir, *potterImportPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize Go modules automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "This usually happens when Potter framework cannot be fetched from GitHub.\n")
		// Определяем путь для go get с учетом версии
		goGetPath := *potterImportPath
		if !strings.Contains(goGetPath, "@") {
			goGetPath = strings.Split(goGetPath, "@")[0] + "@main"
		}
		fmt.Fprintf(os.Stderr, "Please run manually: cd %s && make deps\n", *outputDir)
		fmt.Fprintf(os.Stderr, "Or: cd %s && go get %s && go mod tidy\n", *outputDir, goGetPath)
		modulesInitialized = false
	} else {
		fmt.Printf("Go modules initialized successfully\n")
		modulesInitialized = true
	}

	if modulesInitialized {
		fmt.Printf("Project initialized successfully in %s\n", *outputDir)
	} else {
		fmt.Printf("Project initialized in %s (with warnings - see above)\n", *outputDir)
	}
	fmt.Println("Next steps:")
	fmt.Println("  1. cd", *outputDir)
	fmt.Println("  2. make docker-up")
	fmt.Println("  3. make migrate")
	fmt.Println("  4. make run")
}

func runGenerate() {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	outputDir := fs.String("output", ".", "Output directory")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	potterImportPath := fs.String("potter-import-path", defaultPotterImportPath, "Potter framework import path")

	fs.Parse(os.Args[2:])

	if *protoPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --proto is required\n")
		os.Exit(1)
	}

	if err := validateProtoFile(*protoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Проверка существования файлов
	writer := codegen.NewFileWriter(*outputDir)
	if !*overwrite && writer.FileExists("domain/aggregates.go") {
		fmt.Print("Files already exist. Overwrite? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			return
		}
	}

	// Парсинг proto файла
	spec, err := parseProtoFile(*protoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto file: %v\n", err)
		os.Exit(1)
	}

	// Определение module path
	modulePath := spec.ModuleName
	if modulePath == "" {
		fmt.Fprintf(os.Stderr, "Error: module_name is required in potter.service option\n")
		os.Exit(1)
	}

	config := &codegen.GeneratorConfig{
		ModulePath:      modulePath,
		OutputDir:       *outputDir,
		PackageName:     filepath.Base(modulePath),
		Overwrite:       *overwrite,
		PreserveUserCode: false,
		PotterImportPath: *potterImportPath, // Импорт из main ветки
	}

	// Запуск генераторов
	generators := []codegen.Generator{
		codegen.NewDomainGenerator(*outputDir),
		codegen.NewApplicationGenerator(*outputDir),
		codegen.NewInfrastructureGenerator(*outputDir),
		codegen.NewPresentationGenerator(*outputDir),
		codegen.NewMainGenerator(*outputDir),
	}

		for _, gen := range generators {
			if err := gen.Generate(spec, config); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", gen.Name(), err)
				os.Exit(1)
			}
		}

	// Автоматическая инициализация Go модулей
	var modulesInitialized bool
	if err := initializeGoModules(*outputDir, *potterImportPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize Go modules automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "This usually happens when Potter framework cannot be fetched from GitHub.\n")
		// Определяем путь для go get с учетом версии
		goGetPath := *potterImportPath
		if !strings.Contains(goGetPath, "@") {
			goGetPath = strings.Split(goGetPath, "@")[0] + "@main"
		}
		fmt.Fprintf(os.Stderr, "Please run manually: cd %s && make deps\n", *outputDir)
		fmt.Fprintf(os.Stderr, "Or: cd %s && go get %s && go mod tidy\n", *outputDir, goGetPath)
		modulesInitialized = false
	} else {
		fmt.Printf("Go modules initialized successfully\n")
		modulesInitialized = true
	}

	if modulesInitialized {
		fmt.Println("Code generation completed")
	} else {
		fmt.Println("Code generation completed (with warnings - see above)")
	}
	fmt.Printf("Generated files in: %s\n", *outputDir)
}

func runUpdate() {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	outputDir := fs.String("output", ".", "Output directory")
	interactive := fs.Bool("interactive", false, "Interactive mode")
	noBackup := fs.Bool("no-backup", false, "Don't create backup")

	fs.Parse(os.Args[2:])

	if *protoPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --proto is required\n")
		os.Exit(1)
	}

	// Парсинг proto файла для получения новой спецификации
	newSpec, err := parseProtoFile(*protoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto file: %v\n", err)
		os.Exit(1)
	}

	// Определение module path
	modulePath := newSpec.ModuleName
	if modulePath == "" {
		fmt.Fprintf(os.Stderr, "Error: module_name is required in potter.service option\n")
		os.Exit(1)
	}

	config := &codegen.GeneratorConfig{
		ModulePath:      modulePath,
		OutputDir:       *outputDir,
		PackageName:     filepath.Base(modulePath),
		Overwrite:       false, // При update не перезаписываем сразу
		PreserveUserCode: true,
		PotterImportPath: defaultPotterImportPath,
	}

	// Создание CodeUpdater
	updater := codegen.NewCodeUpdater(*outputDir)

	// Обновление сгенерированных файлов
	changes, err := updater.UpdateGeneratedFiles(newSpec, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating files: %v\n", err)
		os.Exit(1)
	}

	if len(changes) == 0 {
		fmt.Println("No changes detected")
		return
	}

	// Применение изменений
	if *interactive {
		// Интерактивный режим
		if err := updater.InteractiveUpdate(changes); err != nil {
			fmt.Fprintf(os.Stderr, "Error in interactive update: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Автоматический режим
		for _, change := range changes {
			if err := updater.ApplyUpdate(change.Path, change.NewCode, !*noBackup); err != nil {
				fmt.Fprintf(os.Stderr, "Error applying update to %s: %v\n", change.Path, err)
				os.Exit(1)
			}
		}
		fmt.Printf("Applied %d changes\n", len(changes))
	}

	fmt.Println("Update completed")
}

func runCheck() {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	outputDir := fs.String("output", ".", "Output directory")

	fs.Parse(os.Args[2:])

	if *protoPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --proto is required\n")
		os.Exit(1)
	}

	// Парсинг proto файла для получения новой спецификации
	newSpec, err := parseProtoFile(*protoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto file: %v\n", err)
		os.Exit(1)
	}

	// Определение module path
	modulePath := newSpec.ModuleName
	if modulePath == "" {
		fmt.Fprintf(os.Stderr, "Error: module_name is required in potter.service option\n")
		os.Exit(1)
	}

	config := &codegen.GeneratorConfig{
		ModulePath:      modulePath,
		OutputDir:       *outputDir,
		PackageName:     filepath.Base(modulePath),
		Overwrite:       false,
		PreserveUserCode: true,
		PotterImportPath: defaultPotterImportPath,
	}

	// Создание CodeUpdater
	updater := codegen.NewCodeUpdater(*outputDir)

	// Проверка расхождений (без применения изменений)
	changes, err := updater.UpdateGeneratedFiles(newSpec, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking files: %v\n", err)
		os.Exit(1)
	}

	if len(changes) == 0 {
		fmt.Println("✓ No discrepancies found. Code is in sync with proto.")
		os.Exit(0)
	}

	// Есть расхождения - выводим информацию и завершаем с ненулевым кодом
	fmt.Fprintf(os.Stderr, "✗ Found %d file(s) with discrepancies:\n", len(changes))
	for _, change := range changes {
		fmt.Fprintf(os.Stderr, "  - %s\n", change.Path)
	}
	fmt.Fprintf(os.Stderr, "\nRun 'potter-gen update --proto %s --output %s' to apply changes.\n", *protoPath, *outputDir)
	os.Exit(1)
}

func runSDK() {
	fs := flag.NewFlagSet("sdk", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	outputDir := fs.String("output", ".", "Output directory")
	modulePath := fs.String("module", "", "Go module path")

	fs.Parse(os.Args[2:])

	if *protoPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --proto is required\n")
		os.Exit(1)
	}

	// Парсинг proto файла
	spec, err := parseProtoFile(*protoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto file: %v\n", err)
		os.Exit(1)
	}

	// Определение module path
	if *modulePath == "" {
		if spec.ModuleName != "" {
			*modulePath = spec.ModuleName
		} else {
			fmt.Fprintf(os.Stderr, "Error: --module is required or set module_name in potter.service option\n")
			os.Exit(1)
		}
	}

	config := &codegen.GeneratorConfig{
		ModulePath:      *modulePath,
		OutputDir:       *outputDir,
		PotterImportPath: defaultPotterImportPath,
	}

	sdkGen := codegen.NewSDKGenerator(*outputDir)
	if err := sdkGen.Generate(spec, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating SDK: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SDK generated successfully in %s\n", *outputDir)
	fmt.Println("Next steps:")
	fmt.Println("  1. cd", *outputDir)
	fmt.Println("  2. go mod tidy")
	fmt.Println("  3. Use SDK in your application")
}

func runVersion() {
	fmt.Println("potter-gen version 1.2.0")
	fmt.Println("Potter Framework version 1.2.0")
}

// validateProtoImports проверяет содержимое proto файла на наличие неправильного формата импорта
// Собирает предупреждения вместо возврата ошибки, чтобы не блокировать выполнение до protoc
func validateProtoImports(protoPath string, warnings *[]string) error {
	content, err := os.ReadFile(protoPath)
	if err != nil {
		return fmt.Errorf("failed to read proto file: %w", err)
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	for i, line := range lines {
		// Ищем строки с импортом Potter options
		if strings.Contains(line, "import") && strings.Contains(line, "potter/options.proto") {
			// Проверяем, используется ли неправильный формат импорта
			if strings.Contains(line, "github.com/akriventsev/potter/options.proto") {
				warning := fmt.Sprintf("Warning at line %d: неправильный формат импорта - используется полный путь модуля Go вместо относительного пути\n\n"+
					"❌ НЕПРАВИЛЬНО:\n"+
					"   import \"github.com/akriventsev/potter/options.proto\";\n\n"+
					"✅ ПРАВИЛЬНО:\n"+
					"   import \"potter/options.proto\";\n\n"+
					"Рекомендуется исправить импорт в вашем proto файле.", i+1)
				*warnings = append(*warnings, warning)
			}
		}
	}

	return nil
}

// parseProtoImports извлекает все import statements из proto файла
func parseProtoImports(protoPath string) ([]string, error) {
	content, err := os.ReadFile(protoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proto file: %w", err)
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	var imports []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Ищем строки вида: import "path/to/file.proto";
		if strings.HasPrefix(trimmed, "import \"") && strings.HasSuffix(trimmed, "\";") {
			// Извлекаем путь между кавычками
			start := len("import \"")
			end := len(trimmed) - len("\";")
			importPath := trimmed[start:end]
			imports = append(imports, importPath)
		}
	}

	return imports, nil
}

// validateProtoImportsRecursive рекурсивно проверяет импорты в proto файле и всех вложенных файлах
func validateProtoImportsRecursive(protoPath string, protoDir string, visited map[string]bool, warnings *[]string, maxDepth int) error {
	if maxDepth <= 0 {
		return fmt.Errorf("maximum recursion depth reached while validating proto imports")
	}

	// Получаем абсолютный путь
	absPath, err := filepath.Abs(protoPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Проверяем, не посещали ли мы уже этот файл (защита от циклов)
	if visited[absPath] {
		return nil
	}
	visited[absPath] = true

	// Валидация импортов в текущем файле
	if err := validateProtoImports(absPath, warnings); err != nil {
		return err
	}

	// Извлекаем импорты из текущего файла
	imports, err := parseProtoImports(absPath)
	if err != nil {
		return err
	}

	// Рекурсивно проверяем каждый импортированный файл
	for _, importPath := range imports {
		// Пропускаем стандартные импорты и импорты, которые не являются файлами
		if strings.HasPrefix(importPath, "google/") || strings.HasPrefix(importPath, "google.protobuf") {
			continue
		}

		// Разрешаем путь импорта относительно protoDir
		var importAbsPath string
		if filepath.IsAbs(importPath) {
			importAbsPath = importPath
		} else {
			// Пробуем несколько вариантов разрешения пути
			// 1. Относительно директории текущего файла
			currentFileDir := filepath.Dir(absPath)
			candidate1 := filepath.Join(currentFileDir, importPath)
			// 2. Относительно protoDir
			candidate2 := filepath.Join(protoDir, importPath)
			// 3. Относительно protoDir с учетом структуры директорий
			candidate3 := filepath.Join(protoDir, filepath.Dir(importPath), filepath.Base(importPath))

			// Проверяем существование файла
			if _, err := os.Stat(candidate1); err == nil {
				importAbsPath = candidate1
			} else if _, err := os.Stat(candidate2); err == nil {
				importAbsPath = candidate2
			} else if _, err := os.Stat(candidate3); err == nil {
				importAbsPath = candidate3
			} else {
				// Файл не найден, но это не критично - protoc сам проверит
				if os.Getenv("POTTER_DEBUG") == "1" {
					*warnings = append(*warnings, fmt.Sprintf("DEBUG: Could not resolve import path '%s' in file %s", importPath, absPath))
				}
				continue
			}
		}

		// Нормализуем путь
		importAbsPath, err = filepath.Abs(importAbsPath)
		if err != nil {
			continue
		}

		// Рекурсивно проверяем импортированный файл
		if err := validateProtoImportsRecursive(importAbsPath, protoDir, visited, warnings, maxDepth-1); err != nil {
			return err
		}
	}

	return nil
}

// parseProtoFile парсит proto файл и возвращает ParsedSpec
func parseProtoFile(protoPath string) (*codegen.ParsedSpec, error) {
	// Получаем абсолютный путь
	absPath, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Получаем директорию proto файла и имя файла
	protoDir := filepath.Dir(absPath)
	protoFile := filepath.Base(absPath)

	// Валидация импортов перед запуском protoc (с предупреждениями вместо ошибок)
	warnings := []string{}
	visited := make(map[string]bool)
	const maxRecursionDepth = 5
	if err := validateProtoImportsRecursive(absPath, protoDir, visited, &warnings, maxRecursionDepth); err != nil {
		return nil, fmt.Errorf("failed to validate proto imports: %w", err)
	}

	// Выводим предупреждения, если они есть
	if len(warnings) > 0 {
		for _, warning := range warnings {
			fmt.Fprintln(os.Stderr, warning)
		}
		if os.Getenv("POTTER_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "DEBUG: Found %d warning(s), proceeding to protoc...\n", len(warnings))
		}
	}

	// Находим путь к api/proto (где находятся Potter options)
	// Поднимаемся вверх от protoDir, пока не найдем api/proto/potter/options.proto
	potterOptionsPath := ""
	currentDir := protoDir
	
	// Проверяем переменную окружения POTTER_PROTO_PATH
	if envPath := os.Getenv("POTTER_PROTO_PATH"); envPath != "" {
		testPath := filepath.Join(envPath, "potter", "options.proto")
		if _, err := os.Stat(testPath); err == nil {
			potterOptionsPath = envPath
			if os.Getenv("POTTER_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "DEBUG: Found Potter options via POTTER_PROTO_PATH: %s\n", potterOptionsPath)
			}
		}
	}
	
	// Если не нашли через переменную окружения, ищем вверх по директориям
	if potterOptionsPath == "" {
		for {
			testPath := filepath.Join(currentDir, "api", "proto", "potter", "options.proto")
			if _, err := os.Stat(testPath); err == nil {
				potterOptionsPath = filepath.Join(currentDir, "api", "proto")
				if os.Getenv("POTTER_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "DEBUG: Found Potter options by walking up directories: %s\n", potterOptionsPath)
				}
				break
			}
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				// Достигли корня файловой системы
				break
			}
			currentDir = parentDir
		}
	}
	
	// Fallback: пытаемся найти через go list (если Potter установлен как зависимость)
	if potterOptionsPath == "" {
		cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/akriventsev/potter")
		output, err := cmd.Output()
		if err == nil {
			potterDir := strings.TrimSpace(string(output))
			if potterDir != "" {
				testPath := filepath.Join(potterDir, "api", "proto", "potter", "options.proto")
				if _, err := os.Stat(testPath); err == nil {
					potterOptionsPath = filepath.Join(potterDir, "api", "proto")
					if os.Getenv("POTTER_DEBUG") == "1" {
						fmt.Fprintf(os.Stderr, "DEBUG: Found Potter options via go list: %s\n", potterOptionsPath)
					}
				}
			}
		}
	}
	
	// Fallback: проверяем стандартные пути Go modules cache
	if potterOptionsPath == "" {
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			testPath := filepath.Join(gopath, "pkg", "mod", "github.com", "akriventsev", "potter@*", "api", "proto", "potter", "options.proto")
			matches, _ := filepath.Glob(testPath)
			if len(matches) > 0 {
				potterOptionsPath = filepath.Dir(filepath.Dir(matches[0]))
				if os.Getenv("POTTER_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "DEBUG: Found Potter options in GOPATH: %s\n", potterOptionsPath)
				}
			}
		}
	}

	// Создаем временный файл для descriptor set
	tmpFile, err := os.CreateTemp("", "potter-desc-*.pb")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Вызываем protoc для генерации descriptor set
	// protoc --proto_path=<dir> --descriptor_set_out=<out> --include_imports <proto_file>
	// Используем абсолютный путь к файлу, чтобы избежать конфликта "Input is shadowed"
	// Добавляем protoDir как --proto_path для разрешения относительных импортов
	protocArgs := []string{
		"--proto_path=.",
		"--proto_path=" + protoDir,
		"--descriptor_set_out=" + tmpFile.Name(),
		"--include_imports",
		absPath,
	}
	// Добавляем путь к Potter options, если он найден
	if potterOptionsPath != "" {
		protocArgs = append(protocArgs[:2], append([]string{"--proto_path=" + potterOptionsPath}, protocArgs[2:]...)...)
	}
	cmd := exec.Command("protoc", protocArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("failed to run protoc: %w\nOutput: %s", err, string(output))
		outputStr := string(output)
		
		// Проверка на неправильный формат импорта в выводе protoc
		if strings.Contains(outputStr, "github.com/akriventsev/potter/options.proto") {
			errorMsg += "\n\n❌ Обнаружен неправильный формат импорта!\n\n"
			errorMsg += "В вашем proto файле используется полный путь модуля Go вместо относительного пути.\n\n"
			errorMsg += "❌ НЕПРАВИЛЬНО:\n"
			errorMsg += "   import \"github.com/akriventsev/potter/options.proto\";\n\n"
			errorMsg += "✅ ПРАВИЛЬНО:\n"
			errorMsg += "   import \"potter/options.proto\";\n\n"
			errorMsg += "Исправьте импорт в вашем proto файле и попробуйте снова.\n"
			errorMsg += "protoc ищет файлы относительно --proto_path, а не по полному пути модуля Go."
		} else if potterOptionsPath == "" && strings.Contains(outputStr, "potter/options.proto") {
			// Если Potter options не найден, добавляем полезные инструкции
			errorMsg += "\n\nPotter options file not found. Please ensure one of the following:\n"
			errorMsg += "  1. Run the command from the Potter project directory\n"
			errorMsg += "  2. Set POTTER_PROTO_PATH environment variable: export POTTER_PROTO_PATH=/path/to/potter/api/proto\n"
			errorMsg += "  3. Ensure Potter is installed as a dependency in go.mod\n"
			errorMsg += "\nRecommended import in your proto file: import \"potter/options.proto\";"
		}
		
		return nil, fmt.Errorf(errorMsg)
	}

	// Читаем descriptor set
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read descriptor set: %w", err)
	}

	// Парсим FileDescriptorSet
	fdSet := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, fdSet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal descriptor set: %w", err)
	}

	// Находим нужный файл (тот, который был указан в protoPath)
	var targetFile *descriptorpb.FileDescriptorProto
	relProtoPath, _ := filepath.Rel(protoDir, absPath)
	relProtoPath = strings.ReplaceAll(relProtoPath, "\\", "/")
	
	for _, fd := range fdSet.File {
		fdPath := strings.ReplaceAll(fd.GetName(), "\\", "/")
		if fdPath == relProtoPath || fdPath == protoFile || fd.GetName() == protoFile {
			targetFile = fd
			break
		}
	}

	if targetFile == nil && len(fdSet.File) > 0 {
		// Если не нашли, берем последний файл (обычно это основной файл)
		targetFile = fdSet.File[len(fdSet.File)-1]
	}

	if targetFile == nil {
		return nil, fmt.Errorf("proto file not found in descriptor set")
	}

	// Парсим через ProtoParser
	parser := codegen.NewProtoParser()
	spec, err := parser.ParseProtoFile(targetFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file: %w", err)
	}

	return spec, nil
}

// validateProtoFile валидирует proto файл
func validateProtoFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("proto file not found: %s", path)
	}
	return nil
}

// ensureOutputDir создает output директорию
func ensureOutputDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// runCommand выполняет команду в указанной директории
func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// initializeGoModules инициализирует Go модули после генерации кода
func initializeGoModules(outputDir, potterImportPath string) error {
	// Извлекаем базовый путь без версии для использования в импортах
	baseImportPath := strings.Split(potterImportPath, "@")[0]
	
	// Определяем путь с версией для go get
	// Если potterImportPath уже содержит @version/branch, используем его
	// Иначе добавляем @main как дефолт
	var goGetPath string
	if strings.Contains(potterImportPath, "@") {
		goGetPath = potterImportPath
	} else {
		goGetPath = baseImportPath + "@main"
	}
	
	// Выполняем go get для Potter framework
	fmt.Printf("Initializing Go modules in %s...\n", outputDir)
	fmt.Printf("Running: go get %s\n", goGetPath)
	if err := runCommand(outputDir, "go", "get", goGetPath); err != nil {
		return fmt.Errorf("failed to run 'go get %s': %w", goGetPath, err)
	}
	fmt.Printf("Successfully fetched Potter framework from %s\n", goGetPath)
	fmt.Println("Running: go mod tidy")
	
	// Выполняем go mod tidy для разрешения всех зависимостей
	if err := runCommand(outputDir, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("failed to run 'go mod tidy': %w", err)
	}
	
	return nil
}

