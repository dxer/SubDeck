#!/usr/bin/env bash
set -euo pipefail

REPO="dxer/SubDeck"
BIN_PATH="/usr/local/bin/subdeck"
CONFIG_DIR="/etc/subdeck"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
DATA_DIR="/var/lib/subdeck"
SERVICE_FILE="/etc/systemd/system/subdeck.service"

VERSION="${SUBDECK_VERSION:-}"
PORT="${SUBDECK_PORT:-}"
ADMIN_USER="${SUBDECK_ADMIN_USER:-}"
ADMIN_PASS="${SUBDECK_ADMIN_PASS:-}"
GH_PROXY="${GH_PROXY:-}"
ACTION="install"
PURGE="0"
FORCE="0"
TMP_DIR=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { printf "${CYAN}[SubDeck]${NC} %s\n" "$1"; }
log_ok()    { printf "${GREEN}[SubDeck]${NC} %s\n" "$1"; }
log_warn()  { printf "${YELLOW}[SubDeck]${NC} %s\n" "$1"; }
log_error() { printf "${RED}[SubDeck]${NC} %s\n" "$1" >&2; }

cleanup() {
  if [[ -n "${TMP_DIR}" && -d "${TMP_DIR}" ]]; then
    rm -rf "${TMP_DIR}"
  fi
}
trap cleanup EXIT

usage() {
  cat <<EOF
SubDeck Linux 一键安装/升级/卸载脚本

用法:
  bash install.sh [选项]

选项:
  --port <端口>        指定监听端口 (仅首次安装生效，默认: 8080)
  --user <用户名>      指定管理员用户名 (仅首次安装生效，默认: admin)
  --pass <密码>        指定管理员密码 (仅首次安装生效，留空则随机生成)
  --version <版本>     指定安装/升级版本 Tag，如 v0.1.0 (默认: latest)
  --proxy <前缀URL>    GitHub 下载镜像前缀 (可选，如 https://ghproxy.net/)
  --upgrade            升级到指定或最新版本（保留原配置与数据库）
  --force              即使版本号相同也强制重新下载安装
  --uninstall          卸载 SubDeck 服务与二进制文件
  --purge              配合 --uninstall 使用，同时删除配置与数据目录
  -h, --help           显示此帮助信息
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)      PORT="${2:-}"; shift 2 ;;
    --user)      ADMIN_USER="${2:-}"; shift 2 ;;
    --pass)      ADMIN_PASS="${2:-}"; shift 2 ;;
    --version)   VERSION="${2:-}"; shift 2 ;;
    --proxy)     GH_PROXY="${2:-}"; shift 2 ;;
    --upgrade)   ACTION="install"; shift ;;
    --force)     FORCE="1"; shift ;;
    --uninstall) ACTION="uninstall"; shift ;;
    --purge)     PURGE="1"; shift ;;
    -h|--help)   usage; exit 0 ;;
    *)           log_error "未知参数: $1"; usage; exit 1 ;;
  esac
done

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    log_error "请使用 root 权限运行此脚本 (例如: sudo bash install.sh)"
    exit 1
  fi
}

do_uninstall() {
  log_info "正在卸载 SubDeck..."
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop subdeck 2>/dev/null || true
    systemctl disable subdeck 2>/dev/null || true
    if [[ -f "${SERVICE_FILE}" ]]; then
      rm -f "${SERVICE_FILE}"
      systemctl daemon-reload 2>/dev/null || true
    fi
  fi
  rm -f "${BIN_PATH}" "${BIN_PATH}.bak"

  if [[ "${PURGE}" == "1" ]]; then
    rm -rf "${CONFIG_DIR}" "${DATA_DIR}"
    log_ok "SubDeck 已彻底卸载（含配置文件与数据库）。"
  else
    log_ok "SubDeck 程序与系统服务已移除。"
    log_info "配置与数据已保留在: ${CONFIG_DIR} 与 ${DATA_DIR}（如需一并清理请加 --purge）。"
  fi
}

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "${machine}" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      log_error "暂不支持的 CPU 架构: ${machine} (仅支持 amd64 / arm64)"
      exit 1
      ;;
  esac
}

detect_os() {
  local os
  os="$(uname -s)"
  if [[ "${os}" != "Linux" ]]; then
    log_error "此脚本仅支持 Linux 系统，当前系统: ${os}"
    exit 1
  fi
}

get_installed_version() {
  if [[ -x "${BIN_PATH}" ]]; then
    local out
    out="$("${BIN_PATH}" -version 2>/dev/null || true)"
    printf '%s' "${out}" | awk '{print $2}'
  fi
}

fetch_latest_version() {
  local api_url="https://api.github.com/repos/${REPO}/releases/latest"
  local resp
  if command -v curl >/dev/null 2>&1; then
    resp="$(curl -fsSL "${api_url}" 2>/dev/null || true)"
  elif command -v wget >/dev/null 2>&1; then
    resp="$(wget -qO- "${api_url}" 2>/dev/null || true)"
  else
    log_error "缺少 curl 或 wget，请先安装其中一个。"
    exit 1
  fi

  local tag
  tag="$(printf '%s' "${resp}" | grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*"([^"]+)"$/\1/')"
  if [[ -z "${tag}" ]]; then
    log_error "未能获取最新 Release 版本（请确认仓库已有发布的 Tag，或使用 --version 指定版本）。"
    exit 1
  fi
  echo "${tag}"
}

download_file() {
  local url="$1"
  local dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 10 -o "${dest}" "${url}"
  else
    wget -q --tries=3 --timeout=15 -O "${dest}" "${url}"
  fi
}

gen_random_hex() {
  local bytes="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "${bytes}"
  else
    od -An -N "${bytes}" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

gen_random_password() {
  local raw=""
  raw="$(dd if=/dev/urandom bs=64 count=1 2>/dev/null | tr -dc 'A-Za-z0-9' || true)"
  if [[ "${#raw}" -ge 12 ]]; then
    printf '%s' "${raw:0:12}"
  else
    printf 'SubDeck%s' "$(date +%s | tail -c 6)"
  fi
}

prompt_tty() {
  local prompt="$1"
  local default_val="$2"
  local input=""
  if [[ -r /dev/tty ]]; then
    printf "%s" "${prompt}" >/dev/tty
    IFS= read -r input </dev/tty || true
  fi
  if [[ -z "${input}" ]]; then
    echo "${default_val}"
  else
    echo "${input}"
  fi
}

yaml_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  printf '%s' "${s}"
}

verify_checksum() {
  local archive_path="$1"
  local checksum_file="$2"
  local archive_name
  archive_name="$(basename "${archive_path}")"

  local expected
  expected="$(grep -E "[[:space:]]${archive_name}\$" "${checksum_file}" | awk '{print $1}' | head -n1 || true)"
  if [[ -z "${expected}" ]]; then
    log_warn "checksums.txt 中未找到 ${archive_name} 的校验值，跳过校验。"
    return 0
  fi

  local actual=""
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${archive_path}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "${archive_path}" | awk '{print $1}')"
  else
    log_warn "系统缺少 sha256sum/shasum，跳过校验。"
    return 0
  fi

  if [[ "${actual}" != "${expected}" ]]; then
    log_error "SHA256 校验失败！期望: ${expected}，实际: ${actual}"
    exit 1
  fi
  log_ok "安装包 SHA256 校验通过。"
}

get_public_ip() {
  local ip=""
  if command -v curl >/dev/null 2>&1; then
    ip="$(curl -fsS --max-time 3 https://api.ipify.org 2>/dev/null || true)"
  fi
  if [[ -z "${ip}" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")"
  fi
  echo "${ip:-127.0.0.1}"
}

do_install() {
  detect_os
  local arch
  arch="$(detect_arch)"

  local current_version
  current_version="$(get_installed_version || true)"

  if [[ -z "${VERSION}" ]]; then
    log_info "正在查询最新 Release 版本..."
    VERSION="$(fetch_latest_version)"
  fi

  if [[ -n "${current_version}" ]]; then
    if [[ "${current_version}" == "${VERSION}" && "${FORCE}" != "1" ]]; then
      log_ok "当前已安装最新版本 ${current_version}，无需重复安装（如需强制重装请加 --force）。"
      exit 0
    fi
    log_info "检测到已安装版本 ${current_version}，准备升级至 ${VERSION} (${arch})..."
  else
    log_info "目标版本: ${VERSION} (${arch})"
  fi

  local archive_name="subdeck-linux-${arch}.tar.gz"
  local base_url="https://github.com/${REPO}/releases/download/${VERSION}"
  if [[ -n "${GH_PROXY}" ]]; then
    GH_PROXY="${GH_PROXY%/}/"
    base_url="${GH_PROXY}${base_url}"
  fi

  TMP_DIR="$(mktemp -d)"

  log_info "正在下载 ${archive_name}..."
  download_file "${base_url}/${archive_name}" "${TMP_DIR}/${archive_name}"
  if download_file "${base_url}/checksums.txt" "${TMP_DIR}/checksums.txt" 2>/dev/null; then
    verify_checksum "${TMP_DIR}/${archive_name}" "${TMP_DIR}/checksums.txt"
  else
    log_warn "未获取到 checksums.txt，跳过校验。"
  fi

  tar -xzf "${TMP_DIR}/${archive_name}" -C "${TMP_DIR}"
  if [[ ! -f "${TMP_DIR}/subdeck" ]]; then
    log_error "压缩包中未找到 subdeck 可执行文件。"
    exit 1
  fi

  if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet subdeck 2>/dev/null; then
    log_info "正在停止运行中的 subdeck 服务以便替换文件..."
    systemctl stop subdeck
  fi

  if [[ -f "${BIN_PATH}" ]]; then
    cp -f "${BIN_PATH}" "${BIN_PATH}.bak"
  fi
  install -m 0755 "${TMP_DIR}/subdeck" "${BIN_PATH}"
  mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"
  chmod 0750 "${CONFIG_DIR}" "${DATA_DIR}"

  local fresh_config="0"
  if [[ ! -f "${CONFIG_FILE}" ]]; then
    fresh_config="1"
    if [[ -z "${PORT}" ]]; then
      PORT="$(prompt_tty "请输入服务监听端口 [默认 8080]: " "8080")"
    fi
    if ! [[ "${PORT}" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
      log_error "无效端口号: ${PORT}"
      exit 1
    fi

    if [[ -z "${ADMIN_USER}" ]]; then
      ADMIN_USER="$(prompt_tty "请输入管理员用户名 [默认 admin]: " "admin")"
    fi
    if [[ -z "${ADMIN_PASS}" ]]; then
      local default_pass
      default_pass="$(gen_random_password)"
      ADMIN_PASS="$(prompt_tty "请输入管理员密码 [留空自动随机生成]: " "${default_pass}")"
    fi

    local jwt_secret
    jwt_secret="$(gen_random_hex 32)"

    cat > "${CONFIG_FILE}" <<EOF
# SubDeck 配置文件
# 修改后执行: systemctl restart subdeck

server:
  host: "0.0.0.0"
  port: ${PORT}
  trusted_proxies: []
  cors_origin: "*"

database:
  path: "${DATA_DIR}/subs.db"

auth:
  admin_username: "$(yaml_escape "${ADMIN_USER}")"
  admin_password: "$(yaml_escape "${ADMIN_PASS}")"
  jwt_secret: "${jwt_secret}"
EOF
    chmod 0600 "${CONFIG_FILE}"
    log_ok "已生成配置文件: ${CONFIG_FILE}"
  else
    log_info "检测到已存在配置文件 ${CONFIG_FILE}，保留原配置与数据库。"
    PORT="$(grep -E '^[[:space:]]*port:' "${CONFIG_FILE}" | head -n1 | awk '{print $2}' | tr -d '"' || true)"
    PORT="${PORT:-8080}"
  fi

  if command -v systemctl >/dev/null 2>&1; then
    cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=SubDeck - Lightweight VPN Subscription Control Plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${DATA_DIR}
ExecStart=${BIN_PATH} -config ${CONFIG_FILE}
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable subdeck >/dev/null 2>&1 || true
    if systemctl restart subdeck; then
      sleep 1
      if systemctl is-active --quiet subdeck; then
        rm -f "${BIN_PATH}.bak"
        log_ok "SubDeck 服务已启动并设置开机自启！"
      else
        log_warn "服务启动异常，请查看日志: journalctl -u subdeck -n 50 --no-pager"
      fi
    else
      log_error "服务启动失败，请查看日志: journalctl -u subdeck -n 50 --no-pager"
      exit 1
    fi
  else
    rm -f "${BIN_PATH}.bak"
    log_warn "当前环境未检测到 systemd，请手动运行: ${BIN_PATH} -config ${CONFIG_FILE}"
  fi

  local ip
  ip="$(get_public_ip)"
  echo ""
  echo "====================================================="
  if [[ -n "${current_version}" ]]; then
    echo "  SubDeck 升级完成 (${current_version} -> ${VERSION})"
  else
    echo "  SubDeck 安装完成 (${VERSION})"
  fi
  echo "====================================================="
  echo "  访问地址 : http://${ip}:${PORT}"
  echo "  配置文件 : ${CONFIG_FILE}"
  echo "  数据目录 : ${DATA_DIR}"
  if [[ "${fresh_config}" == "1" ]]; then
    echo "  管理员   : ${ADMIN_USER}"
    echo "  初始密码 : ${ADMIN_PASS}"
  fi
  echo "-----------------------------------------------------"
  echo "  常用命令 :"
  echo "    查看状态 : systemctl status subdeck"
  echo "    重启生效 : systemctl restart subdeck"
  echo "    查看日志 : journalctl -u subdeck -f"
  echo "    一键升级 : curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sudo bash -s -- --upgrade"
  echo "    卸载服务 : curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sudo bash -s -- --uninstall"
  echo "====================================================="
}

require_root
if [[ "${ACTION}" == "uninstall" ]]; then
  do_uninstall
else
  do_install
fi
