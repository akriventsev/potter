package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"github.com/akriventsev/potter/framework/codegen"
)

func main() {
	var flags flag.FlagSet
	modulePath := flags.String("module", "", "Go module path")
	outputDir := flags.String("output", ".", "Output directory")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		return generate(gen, *modulePath, *outputDir)
	})
}

func generate(gen *protogen.Plugin, modulePath, outputDir string) error {
	parser := codegen.NewProtoParser()
	
	// Собираем все GeneratedFile для возврата через protogen
	type fileContent struct {
		path    string
		content []byte
		isGo    bool
	}
	var filesToGenerate []fileContent

	for _, file := range gen.Files {
		if !file.Generate {
			continue
		}

		// file.Proto уже содержит полный FileDescriptorProto со всеми Messages, Services и т.д.
		// Используем ParseProtogenFile для парсинга
		spec, err := parser.ParseProtogenFile(file)
		if err != nil {
			return fmt.Errorf("failed to parse proto file %s: %w", file.Desc.Path(), err)
		}

		// Определение module path
		if modulePath == "" {
			modulePath = spec.ModuleName
			if modulePath == "" {
				// Пытаемся извлечь из go_package опции
				goPkg := string(file.GoPackageName)
				if goPkg != "" {
					// Извлекаем путь модуля из go_package (например, "example/api" -> "example")
					parts := strings.Split(goPkg, "/")
					if len(parts) > 0 {
						modulePath = parts[0]
					}
				}
				if modulePath == "" {
					return fmt.Errorf("module path is required. Set --potter_opt=module=myapp or add potter.service option with module_name")
				}
			}
		}

		// Создание временной директории для генерации
		tempDir, err := os.MkdirTemp("", "potter-gen-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		config := &codegen.GeneratorConfig{
			ModulePath:      modulePath,
			OutputDir:       tempDir,
			PackageName:     string(file.GoPackageName),
			Overwrite:       true,
			PreserveUserCode: false,
		}

		// Запуск генераторов
		generators := []codegen.Generator{
			codegen.NewDomainGenerator(tempDir),
			codegen.NewApplicationGenerator(tempDir),
			codegen.NewInfrastructureGenerator(tempDir),
			codegen.NewPresentationGenerator(tempDir),
			codegen.NewMainGenerator(tempDir),
		}

		for _, g := range generators {
			if err := g.Generate(spec, config); err != nil {
				return fmt.Errorf("failed to generate %s: %w", g.Name(), err)
			}
		}

		// Собираем все сгенерированные файлы
		err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			// Определяем тип файла
			isGo := strings.HasSuffix(path, ".go")
			isSQL := strings.HasSuffix(path, ".sql")
			isYAML := strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
			isMD := strings.HasSuffix(path, ".md")
			isMod := strings.HasSuffix(path, ".mod")
			isMakefile := strings.HasSuffix(path, "Makefile")
			isEnv := strings.HasSuffix(path, ".env.example")

			if !isGo && !isSQL && !isYAML && !isMD && !isMod && !isMakefile && !isEnv {
				return nil
			}

			relPath, err := filepath.Rel(tempDir, path)
			if err != nil {
				return err
			}

			// Определяем относительный путь для вывода
			outputPath := relPath
			if outputDir != "." && outputDir != "" {
				outputPath = filepath.Join(outputDir, relPath)
			}

			// Читаем содержимое файла
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read generated file %s: %w", path, err)
			}

			filesToGenerate = append(filesToGenerate, fileContent{
				path:    outputPath,
				content: content,
				isGo:    isGo,
			})

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to process generated files: %w", err)
		}
	}

	// Добавляем все файлы через protogen.GeneratedFile
	for _, file := range filesToGenerate {
		// Для Go файлов используем импорт исходного файла (или пустой)
		var importPath protogen.GoImportPath
		if file.isGo {
			// Для Go файлов используем путь модуля + путь файла
			// Но для protogen нужно использовать правильный импорт
			importPath = protogen.GoImportPath(modulePath)
		} else {
			// Для не-Go файлов используем пустой импорт
			importPath = ""
		}

		gfile := gen.NewGeneratedFile(file.path, importPath)
		if _, err := gfile.Write(file.content); err != nil {
			return fmt.Errorf("failed to write to generated file %s: %w", file.path, err)
		}
	}

	return nil
}

