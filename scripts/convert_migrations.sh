#!/bin/bash

# Скрипт для автоматической конвертации миграций из формата Potter v1.3.x в формат goose
# Использование: ./scripts/convert_migrations.sh [OPTIONS] <migrations_dir>
#
# Требования: bash 4.0+ (для поддержки ассоциативных массивов)
# На macOS может потребоваться установка обновленной версии bash через Homebrew:
#   brew install bash
#   Затем изменить shebang на: #!/usr/local/bin/bash
# Или установить bash 4+ в системе и использовать полный путь

set -e

# Проверка версии bash
# Ассоциативные массивы (declare -A) требуют bash 4.0+
check_bash_version() {
    local version_str="${BASH_VERSION:-0.0.0}"
    local major_version=$(echo "$version_str" | cut -d. -f1)
    local minor_version=$(echo "$version_str" | cut -d. -f2)
    
    if [ "$major_version" -lt 4 ]; then
        return 1
    fi
    
    # Проверяем возможность объявления ассоциативного массива
    # Это более надежная проверка, чем просто версия
    if ! (declare -A test_array 2>/dev/null); then
        return 1
    fi
    
    return 0
}

if ! check_bash_version; then
    echo "Error: This script requires bash 4.0 or higher for associative array support." >&2
    echo "Your current bash version: ${BASH_VERSION:-unknown}" >&2
    echo "" >&2
    echo "On macOS, the default bash is version 3.2. You can install a newer version:" >&2
    echo "  brew install bash" >&2
    echo "" >&2
    echo "Then run the script with:" >&2
    echo "  /usr/local/bin/bash $0 [OPTIONS] <migrations_dir>" >&2
    echo "" >&2
    echo "Or update the shebang in this script to:" >&2
    echo "  #!/usr/local/bin/bash" >&2
    exit 1
fi

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Флаги
DRY_RUN=false
NO_BACKUP=false
MIGRATIONS_DIR=""

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --no-backup)
            NO_BACKUP=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS] <migrations_dir>"
            echo ""
            echo "Options:"
            echo "  --dry-run      Preview changes without making them"
            echo "  --no-backup    Skip creating backup of old files"
            echo "  -h, --help     Show this help message"
            echo ""
            echo "Example:"
            echo "  $0 ./migrations"
            echo "  $0 --dry-run ./migrations"
            exit 0
            ;;
        *)
            if [ -z "$MIGRATIONS_DIR" ]; then
                MIGRATIONS_DIR="$1"
            else
                echo -e "${RED}Error: Multiple directories specified${NC}"
                exit 1
            fi
            shift
            ;;
    esac
done

# Проверка аргументов
if [ -z "$MIGRATIONS_DIR" ]; then
    echo -e "${RED}Error: migrations directory is required${NC}"
    echo "Usage: $0 [OPTIONS] <migrations_dir>"
    exit 1
fi

# Проверка существования директории
if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo -e "${RED}Error: Directory '$MIGRATIONS_DIR' does not exist${NC}"
    exit 1
fi

# Функция для извлечения версии и имени из имени файла
extract_version_and_name() {
    local filename="$1"
    local basename=$(basename "$filename" .sql)
    
    # Убираем расширения .up или .down
    basename=$(echo "$basename" | sed 's/\.up$//' | sed 's/\.down$//')
    
    # Извлекаем версию (числа в начале) и имя
    if [[ $basename =~ ^([0-9]+)_(.+)$ ]]; then
        VERSION="${BASH_REMATCH[1]}"
        NAME="${BASH_REMATCH[2]}"
    elif [[ $basename =~ ^([0-9]+)$ ]]; then
        VERSION="${BASH_REMATCH[1]}"
        NAME="migration"
    else
        VERSION=""
        NAME="$basename"
    fi
}

# Функция для создания backup
create_backup() {
    if [ "$NO_BACKUP" = true ]; then
        return
    fi
    
    local backup_dir="${MIGRATIONS_DIR}/.backup"
    if [ "$DRY_RUN" = false ]; then
        mkdir -p "$backup_dir"
        cp "$1" "$backup_dir/" 2>/dev/null || true
    fi
}

# Поиск всех пар .up.sql и .down.sql файлов
declare -A migration_pairs
declare -A processed_files

# Сначала находим все .up.sql файлы
for up_file in "$MIGRATIONS_DIR"/*.up.sql; do
    if [ -f "$up_file" ]; then
        extract_version_and_name "$up_file"
        if [ -n "$VERSION" ]; then
            key="${VERSION}_${NAME}"
            migration_pairs["$key"]="$up_file"
        fi
    fi
done

# Находим соответствующие .down.sql файлы
for down_file in "$MIGRATIONS_DIR"/*.down.sql; do
    if [ -f "$down_file" ]; then
        extract_version_and_name "$down_file"
        if [ -n "$VERSION" ]; then
            key="${VERSION}_${NAME}"
            if [ -n "${migration_pairs[$key]}" ]; then
                # Добавляем down файл к паре
                migration_pairs["${key}_down"]="$down_file"
            fi
        fi
    fi
done

# Конвертация
converted_count=0
skipped_count=0

echo -e "${GREEN}Converting migrations from Potter v1.3.x format to goose format...${NC}"
echo ""

for key in "${!migration_pairs[@]}"; do
    if [[ $key == *_down ]]; then
        continue
    fi
    
    up_file="${migration_pairs[$key]}"
    down_key="${key}_down"
    down_file="${migration_pairs[$down_key]}"
    
    extract_version_and_name "$up_file"
    
    # Формируем имя нового файла
    new_filename="${MIGRATIONS_DIR}/${VERSION}_${NAME}.sql"
    
    # Проверяем, не существует ли уже файл в формате goose
    if [ -f "$new_filename" ]; then
        echo -e "${YELLOW}⚠ Skipping: $new_filename already exists${NC}"
        skipped_count=$((skipped_count + 1))
        continue
    fi
    
    if [ "$DRY_RUN" = true ]; then
        echo -e "${GREEN}[DRY RUN] Would convert:${NC}"
        echo "  Up:   $up_file"
        if [ -n "$down_file" ]; then
            echo "  Down: $down_file"
        else
            echo "  Down: (not found)"
        fi
        echo "  To:   $new_filename"
        echo ""
    else
        # Создаем backup
        create_backup "$up_file"
        if [ -n "$down_file" ]; then
            create_backup "$down_file"
        fi
        
        # Создаем новый файл
        {
            echo "-- +goose Up"
            cat "$up_file"
            echo ""
            echo "-- +goose Down"
            if [ -n "$down_file" ] && [ -f "$down_file" ]; then
                cat "$down_file"
            else
                echo "-- Down migration not found"
            fi
        } > "$new_filename"
        
        echo -e "${GREEN}✓ Converted: $new_filename${NC}"
        converted_count=$((converted_count + 1))
    fi
done

# Обработка одиночных .sql файлов (уже в формате goose или старые миграции)
for sql_file in "$MIGRATIONS_DIR"/*.sql; do
    if [ -f "$sql_file" ]; then
        basename=$(basename "$sql_file")
        # Пропускаем файлы, которые мы только что создали или которые уже обработаны
        if [[ $basename == *.up.sql ]] || [[ $basename == *.down.sql ]]; then
            continue
        fi
        
        # Проверяем, есть ли уже аннотации goose
        if grep -q "^-- +goose Up" "$sql_file" 2>/dev/null; then
            echo -e "${GREEN}✓ Already in goose format: $basename${NC}"
        else
            echo -e "${YELLOW}⚠ File without goose annotations: $basename${NC}"
            echo "  You may need to manually add -- +goose Up and -- +goose Down annotations"
        fi
    fi
done

echo ""
if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}Dry run completed. No files were modified.${NC}"
    echo "Run without --dry-run to apply changes."
else
    echo -e "${GREEN}Conversion completed!${NC}"
    echo "  Converted: $converted_count migration(s)"
    if [ $skipped_count -gt 0 ]; then
        echo "  Skipped: $skipped_count migration(s)"
    fi
    if [ "$NO_BACKUP" = false ]; then
        echo "  Backup created in: ${MIGRATIONS_DIR}/.backup"
    fi
fi

