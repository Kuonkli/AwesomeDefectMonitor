#!/bin/bash

set -e

echo "Generating all protobuf contracts..."

# Определяем корневую директорию проекта
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS_DIR="$PROJECT_ROOT/shared/proto"

echo "Project root: $PROJECT_ROOT"
echo "Contracts dir: $CONTRACTS_DIR"

cd "$PROJECT_ROOT/shared"
rm -rf protogen/*

# Создаем структуру папок
mkdir -p protogen/user
mkdir -p protogen/project

# Генерируем для каждого прото-файла отдельно
for proto_file in "$CONTRACTS_DIR"/*/*.proto; do
    if [ -f "$proto_file" ]; then
        echo "Generating for: $proto_file"

        # Получаем имя папки (сервиса)
        service_name=$(basename $(dirname "$proto_file"))

        protoc --proto_path="$CONTRACTS_DIR" \
               --go_out=protogen \
               --go_opt=module=awesome-defect-tracker/shared \
               --go-grpc_out=protogen \
               --go-grpc_opt=module=awesome-defect-tracker/shared \
               "$proto_file"
    fi
done

echo "All contracts generated successfully!"