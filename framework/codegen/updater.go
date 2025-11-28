package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CodeUpdater система обновления существующего кода
type CodeUpdater struct {
	outputDir string
}

// NewCodeUpdater создает новый CodeUpdater
func NewCodeUpdater(outputDir string) *CodeUpdater {
	return &CodeUpdater{outputDir: outputDir}
}

// ParsedFile структурированное представление Go файла
type ParsedFile struct {
	Package       string
	Imports       []Import
	Types         []TypeDecl
	Functions     []FunctionDecl
	UserCodeBlocks map[string]string // marker -> code
}

// Import информация об импорте
type Import struct {
	Alias string
	Path  string
}

// TypeDecl декларация типа
type TypeDecl struct {
	Name   string
	Kind   string // struct, interface, etc.
	Fields []FieldDecl
}

// FieldDecl декларация поля
type FieldDecl struct {
	Name string
	Type string
	Tag  string
}

// FunctionDecl декларация функции
type FunctionDecl struct {
	Name       string
	Receiver   string
	Parameters []Parameter
	Returns    []Return
	Body       string
}

// Parameter параметр функции
type Parameter struct {
	Name string
	Type string
}

// Return возвращаемое значение
type Return struct {
	Name string
	Type string
}

// FileChange изменение в файле
type FileChange struct {
	Path    string
	OldCode string
	NewCode string
	Diff    string
}

// ParseExistingFile парсит существующий Go файл
func (u *CodeUpdater) ParseExistingFile(path string) (*ParsedFile, error) {
	fullPath := filepath.Join(u.outputDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fullPath, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	parsed := &ParsedFile{
		Package:        node.Name.Name,
		Imports:        []Import{},
		Types:          []TypeDecl{},
		Functions:      []FunctionDecl{},
		UserCodeBlocks: make(map[string]string),
	}

	// Извлечение импортов
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		parsed.Imports = append(parsed.Imports, Import{
			Alias: alias,
			Path:  importPath,
		})
	}

	// Извлечение типов
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						typeDecl := TypeDecl{
							Name: ts.Name.Name,
						}
						if st, ok := ts.Type.(*ast.StructType); ok {
							typeDecl.Kind = "struct"
							for _, field := range st.Fields.List {
								if len(field.Names) > 0 {
									typeDecl.Fields = append(typeDecl.Fields, FieldDecl{
										Name: field.Names[0].Name,
										Type: u.typeToString(field.Type),
									})
								}
							}
						}
						parsed.Types = append(parsed.Types, typeDecl)
					}
				}
			}
		case *ast.FuncDecl:
			funcDecl := FunctionDecl{
				Name: x.Name.Name,
			}
			if x.Recv != nil && len(x.Recv.List) > 0 {
				funcDecl.Receiver = u.typeToString(x.Recv.List[0].Type)
			}
			parsed.Functions = append(parsed.Functions, funcDecl)
		}
		return true
	})

	// Извлечение пользовательского кода
	parsed.UserCodeBlocks = u.ExtractUserCode(string(data))

	return parsed, nil
}

// ExtractUserCode извлекает блоки пользовательского кода
func (u *CodeUpdater) ExtractUserCode(fileContent string) map[string]string {
	blocks := make(map[string]string)

	// Паттерн для поиска маркеров USER CODE BEGIN/END
	pattern := regexp.MustCompile(`//\s*USER CODE BEGIN:\s*(\w+)\s*\n(.*?)\n//\s*USER CODE END:\s*\1`)
	matches := pattern.FindAllStringSubmatch(fileContent, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			marker := match[1]
			code := match[2]
			blocks[marker] = code
		}
	}

	return blocks
}

// MergeWithUserCode вставляет пользовательский код в сгенерированный
func (u *CodeUpdater) MergeWithUserCode(generated, userCode string, marker string) string {
	// Поиск маркера в сгенерированном коде
	beginPattern := regexp.MustCompile(fmt.Sprintf(`//\s*USER CODE BEGIN:\s*%s\s*\n`, regexp.QuoteMeta(marker)))
	endPattern := regexp.MustCompile(fmt.Sprintf(`//\s*USER CODE END:\s*%s\s*\n`, regexp.QuoteMeta(marker)))

	beginMatch := beginPattern.FindStringIndex(generated)
	endMatch := endPattern.FindStringIndex(generated)

	if beginMatch != nil && endMatch != nil {
		// Замена содержимого между маркерами
		before := generated[:beginMatch[1]]
		after := generated[endMatch[0]:]
		return before + userCode + "\n" + after
	}

	return generated
}

// UpdateMethodSignatures обновляет сигнатуры методов
func (u *CodeUpdater) UpdateMethodSignatures(oldFile, newFile *ParsedFile) error {
	// Создаем карту старых методов по имени
	oldMethods := make(map[string]*FunctionDecl)
	for i := range oldFile.Functions {
		m := &oldFile.Functions[i]
		key := u.getMethodKey(m)
		oldMethods[key] = m
	}

	// Проходим по новым методам и обновляем при необходимости
	for i := range newFile.Functions {
		newMethod := &newFile.Functions[i]
		key := u.getMethodKey(newMethod)
		
		if oldMethod, exists := oldMethods[key]; exists {
			// Метод существует, проверяем изменилась ли сигнатура
			if u.hasSignatureChanged(oldMethod, newMethod) {
				// Сигнатура изменилась - сохраняем тело из старого метода
				newMethod.Body = oldMethod.Body
				// Добавляем TODO комментарий
				if newMethod.Body != "" && !strings.Contains(newMethod.Body, "TODO") {
					newMethod.Body = "// TODO: check updated signature\n\t" + newMethod.Body
				}
			} else {
				// Сигнатура не изменилась - сохраняем тело
				newMethod.Body = oldMethod.Body
			}
		}
		// Новые методы оставляем с пустым телом (будут заполнены генератором)
	}

	// Отмечаем удаленные методы как DEPRECATED
	for key, oldMethod := range oldMethods {
		found := false
		for _, newMethod := range newFile.Functions {
			if u.getMethodKey(&newMethod) == key {
				found = true
				break
			}
		}
		if !found {
			// Метод удален - добавляем как DEPRECATED
			deprecatedMethod := *oldMethod
			deprecatedMethod.Body = "// DEPRECATED: This method has been removed in the new version\n\tpanic(\"method deprecated\")"
			newFile.Functions = append(newFile.Functions, deprecatedMethod)
		}
	}

	return nil
}

// getMethodKey создает ключ для метода (имя + ресивер)
func (u *CodeUpdater) getMethodKey(m *FunctionDecl) string {
	if m.Receiver != "" {
		return fmt.Sprintf("%s.%s", m.Receiver, m.Name)
	}
	return m.Name
}

// hasSignatureChanged проверяет изменилась ли сигнатура метода
func (u *CodeUpdater) hasSignatureChanged(old, new *FunctionDecl) bool {
	// Сравниваем параметры и возвращаемые значения
	if len(old.Parameters) != len(new.Parameters) {
		return true
	}
	if len(old.Returns) != len(new.Returns) {
		return true
	}

	// Сравниваем параметры
	for i, oldParam := range old.Parameters {
		if i >= len(new.Parameters) {
			return true
		}
		newParam := new.Parameters[i]
		if oldParam.Type != newParam.Type {
			return true
		}
	}

	// Сравниваем возвращаемые значения
	for i, oldRet := range old.Returns {
		if i >= len(new.Returns) {
			return true
		}
		newRet := new.Returns[i]
		if oldRet.Type != newRet.Type {
			return true
		}
	}

	return false
}

// CreateBackup создает backup файла
func (u *CodeUpdater) CreateBackup(path string) error {
	fullPath := filepath.Join(u.outputDir, path)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fullPath + "." + timestamp + ".backup"

	return os.WriteFile(backupPath, data, 0644)
}

// UpdateGeneratedFiles обновляет сгенерированные файлы, сохраняя пользовательский код
func (u *CodeUpdater) UpdateGeneratedFiles(spec *ParsedSpec, config *GeneratorConfig) ([]FileChange, error) {
	var changes []FileChange

	// Создаем временную директорию для генерации нового кода
	tempDir, err := os.MkdirTemp("", "potter-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Создаем генераторы
	domainGen := NewDomainGenerator(tempDir)
	appGen := NewApplicationGenerator(tempDir)

	// Генерируем все файлы во временную директорию
	if err := domainGen.Generate(spec, config); err != nil {
		return nil, fmt.Errorf("failed to generate domain: %w", err)
	}
	if err := appGen.Generate(spec, config); err != nil {
		return nil, fmt.Errorf("failed to generate application: %w", err)
	}

	// Сканируем все .go файлы в outputDir
	err = filepath.Walk(u.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Получаем относительный путь
		relPath, err := filepath.Rel(u.outputDir, path)
		if err != nil {
			return err
		}

		// Пропускаем файлы вне основных директорий
		if !strings.HasPrefix(relPath, "domain/") &&
			!strings.HasPrefix(relPath, "application/") {
			return nil
		}

		// Читаем старый контент
		oldContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Парсим старый файл
		oldParsed, err := u.ParseExistingFile(relPath)
		if err != nil {
			// Если не удалось распарсить, пропускаем
			return nil
		}

		// Извлекаем пользовательский код
		userCode := oldParsed.UserCodeBlocks

		// Проверяем, существует ли новый файл во временной директории
		newPath := filepath.Join(tempDir, relPath)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			// Файл удален в новой версии - пропускаем
			return nil
		}

		// Читаем новый сгенерированный контент
		newContentBytes, err := os.ReadFile(newPath)
		if err != nil {
			return err
		}
		newContent := string(newContentBytes)

		// Парсим новый файл
		newParsed, err := u.parseFileFromContent(newContent, relPath)
		if err != nil {
			// Если не удалось распарсить, используем как есть
		} else {
			// Обновляем сигнатуры методов
			if err := u.UpdateMethodSignatures(oldParsed, newParsed); err != nil {
				// Логируем ошибку, но продолжаем
				_ = err
			}
		}

		// Мержим пользовательский код обратно
		for marker, code := range userCode {
			newContent = u.MergeWithUserCode(newContent, code, marker)
		}

		// Если есть изменения, создаем FileChange
		if newContent != string(oldContent) {
			diff := u.GenerateDiff(string(oldContent), newContent)
			changes = append(changes, FileChange{
				Path:    relPath,
				OldCode: string(oldContent),
				NewCode: newContent,
				Diff:    diff,
			})
		}

		return nil
	})

	return changes, err
}

// parseFileFromContent парсит Go файл из содержимого
func (u *CodeUpdater) parseFileFromContent(content, path string) (*ParsedFile, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, []byte(content), parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	parsed := &ParsedFile{
		Package:        node.Name.Name,
		Imports:        []Import{},
		Types:          []TypeDecl{},
		Functions:      []FunctionDecl{},
		UserCodeBlocks: make(map[string]string),
	}

	// Извлечение импортов
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		parsed.Imports = append(parsed.Imports, Import{
			Alias: alias,
			Path:  importPath,
		})
	}

	// Извлечение типов и функций
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						typeDecl := TypeDecl{
							Name: ts.Name.Name,
						}
						if st, ok := ts.Type.(*ast.StructType); ok {
							typeDecl.Kind = "struct"
							for _, field := range st.Fields.List {
								if len(field.Names) > 0 {
									typeDecl.Fields = append(typeDecl.Fields, FieldDecl{
										Name: field.Names[0].Name,
										Type: u.typeToString(field.Type),
									})
								}
							}
						}
						parsed.Types = append(parsed.Types, typeDecl)
					}
				}
			}
		case *ast.FuncDecl:
			funcDecl := FunctionDecl{
				Name: x.Name.Name,
			}
			if x.Recv != nil && len(x.Recv.List) > 0 {
				funcDecl.Receiver = u.typeToString(x.Recv.List[0].Type)
			}
			// Извлекаем параметры
			if x.Type != nil && x.Type.Params != nil {
				for _, param := range x.Type.Params.List {
					paramType := u.typeToString(param.Type)
					for _, name := range param.Names {
						funcDecl.Parameters = append(funcDecl.Parameters, Parameter{
							Name: name.Name,
							Type: paramType,
						})
					}
					if len(param.Names) == 0 {
						funcDecl.Parameters = append(funcDecl.Parameters, Parameter{
							Name: "",
							Type: paramType,
						})
					}
				}
			}
			// Извлекаем возвращаемые значения
			if x.Type != nil && x.Type.Results != nil {
				for _, result := range x.Type.Results.List {
					resultType := u.typeToString(result.Type)
					for _, name := range result.Names {
						funcDecl.Returns = append(funcDecl.Returns, Return{
							Name: name.Name,
							Type: resultType,
						})
					}
					if len(result.Names) == 0 {
						funcDecl.Returns = append(funcDecl.Returns, Return{
							Name: "",
							Type: resultType,
						})
					}
				}
			}
			parsed.Functions = append(parsed.Functions, funcDecl)
		}
		return true
	})

	return parsed, nil
}

// GenerateDiff генерирует unified diff с номерами строк и контекстом
func (u *CodeUpdater) GenerateDiff(oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	var diff strings.Builder
	diff.WriteString("--- old\n+++ new\n")
	
	// Константы для unified diff
	const contextLines = 3 // количество контекстных строк вокруг изменений
	
	// Вычисляем различия с помощью простого алгоритма
	// Группируем изменения в hunks с контекстом
	var hunks []hunk
	var currentHunk *hunk
	
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}
	
	oldLineNum := 1
	newLineNum := 1
	
	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		
		if oldLine != newLine {
			// Начало или продолжение hunk
			if currentHunk == nil {
				currentHunk = &hunk{
					oldStart: oldLineNum,
					newStart: newLineNum,
					oldLines: []string{},
					newLines: []string{},
				}
			}
			
			if oldLine != "" {
				currentHunk.oldLines = append(currentHunk.oldLines, oldLine)
				currentHunk.oldEnd = oldLineNum
			}
			if newLine != "" {
				currentHunk.newLines = append(currentHunk.newLines, newLine)
				currentHunk.newEnd = newLineNum
			}
		} else {
			// Неизмененная строка
			if currentHunk != nil {
				// Завершаем текущий hunk
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
		
		if oldLine != "" {
			oldLineNum++
		}
		if newLine != "" {
			newLineNum++
		}
	}
	
	// Добавляем последний hunk, если есть
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	
	// Формируем unified diff с контекстом
	for _, h := range hunks {
		// Вычисляем начало с учетом контекста
		oldContextStart := h.oldStart - contextLines
		if oldContextStart < 1 {
			oldContextStart = 1
		}
		newContextStart := h.newStart - contextLines
		if newContextStart < 1 {
			newContextStart = 1
		}
		
		// Вычисляем конец с учетом контекста
		oldContextEnd := h.oldEnd + contextLines
		if oldContextEnd > len(oldLines) {
			oldContextEnd = len(oldLines)
		}
		newContextEnd := h.newEnd + contextLines
		if newContextEnd > len(newLines) {
			newContextEnd = len(newLines)
		}
		
		// Заголовок hunk
		oldCount := oldContextEnd - oldContextStart + 1
		newCount := newContextEnd - newContextStart + 1
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldContextStart, oldCount, newContextStart, newCount))
		
		// Контекстные строки до изменений
		for i := oldContextStart - 1; i < h.oldStart-1 && i < len(oldLines); i++ {
			if i >= 0 {
				diff.WriteString(fmt.Sprintf(" %s\n", oldLines[i]))
			}
		}
		
		// Измененные строки
		oldHunkIdx := 0
		newHunkIdx := 0
		
		for oldHunkIdx < len(h.oldLines) || newHunkIdx < len(h.newLines) {
			if oldHunkIdx < len(h.oldLines) && newHunkIdx < len(h.newLines) {
				// Измененная строка
				diff.WriteString(fmt.Sprintf("-%s\n", h.oldLines[oldHunkIdx]))
				diff.WriteString(fmt.Sprintf("+%s\n", h.newLines[newHunkIdx]))
				oldHunkIdx++
				newHunkIdx++
			} else if oldHunkIdx < len(h.oldLines) {
				// Удаленная строка
				diff.WriteString(fmt.Sprintf("-%s\n", h.oldLines[oldHunkIdx]))
				oldHunkIdx++
			} else if newHunkIdx < len(h.newLines) {
				// Добавленная строка
				diff.WriteString(fmt.Sprintf("+%s\n", h.newLines[newHunkIdx]))
				newHunkIdx++
			}
		}
		
		// Контекстные строки после изменений
		contextEnd := h.oldEnd + contextLines
		if contextEnd > len(oldLines) {
			contextEnd = len(oldLines)
		}
		for i := h.oldEnd; i < contextEnd && i < len(oldLines); i++ {
			diff.WriteString(fmt.Sprintf(" %s\n", oldLines[i]))
		}
	}
	
	return diff.String()
}

// hunk представляет группу изменений в unified diff
type hunk struct {
	oldStart int
	oldEnd   int
	newStart int
	newEnd   int
	oldLines []string
	newLines []string
}

// ApplyUpdate применяет обновление
func (u *CodeUpdater) ApplyUpdate(path string, newContent string, backup bool) error {
	if backup {
		if err := u.CreateBackup(path); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	fullPath := filepath.Join(u.outputDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(fullPath, []byte(newContent), 0644)
}

// InteractiveUpdate интерактивное применение изменений
func (u *CodeUpdater) InteractiveUpdate(changes []FileChange) error {
	for _, change := range changes {
		fmt.Printf("File: %s\n", change.Path)
		fmt.Printf("Diff:\n%s\n", change.Diff)
		fmt.Print("Apply change? (y/n/d for diff): ")

		var response string
		fmt.Scanln(&response)

		switch response {
		case "y", "Y":
			if err := u.ApplyUpdate(change.Path, change.NewCode, true); err != nil {
				return fmt.Errorf("failed to apply update to %s: %w", change.Path, err)
			}
			fmt.Printf("Applied: %s\n", change.Path)
		case "d", "D":
			fmt.Println(change.Diff)
			fmt.Print("Apply change? (y/n): ")
			fmt.Scanln(&response)
			if response == "y" || response == "Y" {
				if err := u.ApplyUpdate(change.Path, change.NewCode, true); err != nil {
					return fmt.Errorf("failed to apply update to %s: %w", change.Path, err)
				}
			}
		default:
			fmt.Printf("Skipped: %s\n", change.Path)
		}
	}

	return nil
}

// typeToString конвертирует ast.Type в строку
func (u *CodeUpdater) typeToString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return u.typeToString(x.X) + "." + x.Sel.Name
	case *ast.StarExpr:
		return "*" + u.typeToString(x.X)
	case *ast.ArrayType:
		return "[]" + u.typeToString(x.Elt)
	default:
		return "unknown"
	}
}

