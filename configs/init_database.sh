#!/bin/bash

# Gather ETH链数据采集系统数据库初始化脚本
# 支持 PostgreSQL 和 MySQL

set -e

# 配置变量
DB_TYPE=${DB_TYPE:-postgresql}  # postgresql 或 mysql
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}        # PostgreSQL默认5432, MySQL默认3306
DB_NAME=${DB_NAME:-gather_config}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD}

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 输出函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查必需的环境变量
check_requirements() {
    log_info "检查环境变量..."
    
    if [ -z "$DB_PASSWORD" ]; then
        log_error "请设置 DB_PASSWORD 环境变量"
        echo "示例: export DB_PASSWORD=your_password"
        exit 1
    fi
    
    if [ "$DB_TYPE" != "postgresql" ] && [ "$DB_TYPE" != "mysql" ]; then
        log_error "DB_TYPE 必须是 postgresql 或 mysql"
        exit 1
    fi
    
    log_info "环境变量检查完成"
}

# 检查数据库连接
check_connection() {
    log_info "检查数据库连接..."
    
    if [ "$DB_TYPE" = "postgresql" ]; then
        if ! command -v psql &> /dev/null; then
            log_error "psql 客户端未找到，请安装 PostgreSQL 客户端"
            exit 1
        fi
        
        export PGPASSWORD="$DB_PASSWORD"
        if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "SELECT 1;" > /dev/null 2>&1; then
            log_info "PostgreSQL 连接成功"
        else
            log_error "无法连接到 PostgreSQL"
            exit 1
        fi
    else
        if ! command -v mysql &> /dev/null; then
            log_error "mysql 客户端未找到，请安装 MySQL 客户端"
            exit 1
        fi
        
        if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "SELECT 1;" > /dev/null 2>&1; then
            log_info "MySQL 连接成功"
        else
            log_error "无法连接到 MySQL"
            exit 1
        fi
    fi
}

# 创建数据库
create_database() {
    log_info "创建数据库 $DB_NAME..."
    
    if [ "$DB_TYPE" = "postgresql" ]; then
        export PGPASSWORD="$DB_PASSWORD"
        if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1; then
            log_warn "数据库 $DB_NAME 已存在"
        else
            psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;"
            log_info "数据库 $DB_NAME 创建成功"
        fi
    else
        if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "USE $DB_NAME;" > /dev/null 2>&1; then
            log_warn "数据库 $DB_NAME 已存在"
        else
            mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "CREATE DATABASE $DB_NAME CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
            log_info "数据库 $DB_NAME 创建成功"
        fi
    fi
}

# 执行SQL脚本
execute_schema() {
    log_info "执行数据库表结构脚本..."
    
    local script_dir="$(dirname "$0")"
    
    if [ "$DB_TYPE" = "postgresql" ]; then
        local schema_file="$script_dir/database_schema.sql"
        if [ ! -f "$schema_file" ]; then
            log_error "PostgreSQL 脚本文件不存在: $schema_file"
            exit 1
        fi
        
        export PGPASSWORD="$DB_PASSWORD"
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$schema_file"
        log_info "PostgreSQL 表结构创建完成"
    else
        local schema_file="$script_dir/database_schema_mysql.sql"
        if [ ! -f "$schema_file" ]; then
            log_error "MySQL 脚本文件不存在: $schema_file"
            exit 1
        fi
        
        mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" < "$schema_file"
        log_info "MySQL 表结构创建完成"
    fi
}

# 验证安装
verify_installation() {
    log_info "验证数据库安装..."
    
    if [ "$DB_TYPE" = "postgresql" ]; then
        export PGPASSWORD="$DB_PASSWORD"
        table_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';")
    else
        table_count=$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" -sN -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$DB_NAME';")
    fi
    
    table_count=$(echo "$table_count" | tr -d ' ')
    
    if [ "$table_count" -gt 5 ]; then
        log_info "数据库安装验证成功，共创建 $table_count 个表"
    else
        log_error "数据库安装可能失败，只发现 $table_count 个表"
        exit 1
    fi
}

# 显示后续步骤
show_next_steps() {
    log_info "数据库初始化完成！"
    echo ""
    echo "后续步骤："
    echo "1. 更新你的配置文件 configs/config.yaml："
    echo ""
    echo "database:"
    echo "  enabled: true"
    if [ "$DB_TYPE" = "postgresql" ]; then
        echo "  dsn: \"postgres://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable\""
    else
        echo "  dsn: \"$DB_USER:$DB_PASSWORD@tcp($DB_HOST:$DB_PORT)/$DB_NAME?charset=utf8mb4&parseTime=True&loc=Local\""
    fi
    echo ""
    echo "2. 启动 gather 服务："
    echo "   ./gather-collector --config configs/config.yaml"
    echo ""
    echo "3. 通过API管理配置："
    echo "   curl http://localhost:8080/api/v1/config/nodes"
    echo ""
    log_info "数据库配置管理已启用！"
}

# 主函数
main() {
    echo "=================================="
    echo "Gather 数据库配置初始化工具"
    echo "=================================="
    echo ""
    
    check_requirements
    check_connection
    create_database
    execute_schema
    verify_installation
    show_next_steps
}

# 执行主函数
main "$@"