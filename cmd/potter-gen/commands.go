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
	"potter/framework/codegen"
)

func runInit() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	protoPath := fs.String("proto", "", "Path to proto file")
	modulePath := fs.String("module", "", "Go module path")
	outputDir := fs.String("output", ".", "Output directory")

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

	fmt.Printf("Project initialized successfully in %s\n", *outputDir)
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

	fmt.Println("Code generation completed")
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
		ModulePath: *modulePath,
		OutputDir:  *outputDir,
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

	// Создаем временный файл для descriptor set
	tmpFile, err := os.CreateTemp("", "potter-desc-*.pb")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Вызываем protoc для генерации descriptor set
	// protoc --proto_path=<dir> --descriptor_set_out=<out> --include_imports <proto_file>
	cmd := exec.Command("protoc",
		"--proto_path="+protoDir,
		"--proto_path=.",
		"--proto_path="+filepath.Join(filepath.Dir(filepath.Dir(protoDir)), "api", "proto"),
		"--descriptor_set_out="+tmpFile.Name(),
		"--include_imports",
		protoFile,
	)

	cmd.Dir = protoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run protoc: %w\nOutput: %s", err, string(output))
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

