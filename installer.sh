#!/bin/sh

set -e

target=""
githubUrl=""
executable_folder=$(eval echo "~/.local/bin")
config_folder=$(eval echo "$HOME/.config/gocker/theming")
default_theme_url="https://raw.githubusercontent.com/pommee/gocker/main/theming/default.yml"
default_theme_path="${config_folder}/default.yml"

get_arch() {
    case $(uname -m) in
        "x86_64" | "amd64" ) echo "amd64" ;;
        "i386" | "i486" | "i586") echo "386" ;;
        "aarch64" | "arm64" | "arm") echo "arm64" ;;
        "mips64el") echo "mips64el" ;;
        "mips64") echo "mips64" ;;
        "mips") echo "mips" ;;
        *) echo "unknown" ;;
    esac
}

get_os() {
    uname -s | tr '[:upper:]' '[:lower:]'
}

get_latest_release() {
    curl --silent "https://api.github.com/repos/pommee/gocker/releases/latest" |
    grep '"tag_name":' |
    sed -E 's/.*"v([^"]+)".*/\1/'
}

download_default_theme() {
    if [ -f "${default_theme_path}" ]; then
        # [4/4] The default.yml file already exists. Skipping download.
        return
    fi

    echo "[4/4] Downloading default.yml to ${default_theme_path}"
    mkdir -p "${config_folder}"
    curl -L -o "${default_theme_path}" "${default_theme_url}"
}

main() {
    os=$(get_os)
    arch=$(get_arch)
    latest_version=$(get_latest_release)
    file_name="gocker_${latest_version}_${os}_${arch}.tar.gz"
    downloadFolder="${TMPDIR:-/tmp}"
    downloaded_file="${downloadFolder}/${file_name}"

    echo "[1/3] Downloading ${file_name} to ${downloadFolder}"
    asset_path=$(curl -sL "https://api.github.com/repos/pommee/gocker/releases" |
        grep -o "https://github.com/pommee/gocker/releases/download/v${latest_version}/${file_name}" |
        head -n 1)
    asset_uri="${githubUrl}${asset_path}"

    if [ -z "$asset_path" ]; then
        echo "ERROR: Unable to find a release asset called ${file_name}"
        exit 1
    fi

    echo "Downloading from: ${asset_uri}"
    rm -f "${downloaded_file}"
    curl --fail --location --output "${downloaded_file}" "${asset_uri}"

    mkdir -p "${executable_folder}"

    echo "[2/3] Installing ${file_name} to ${executable_folder}"
    tar -xzf "${downloaded_file}" -C "${executable_folder}"
    chmod +x "${executable_folder}/gocker"

    echo "[3/3] gocker was installed successfully to ${executable_folder}"

    download_default_theme

    echo "Manually add the directory to your \$HOME/.bash_profile (or similar):"
    echo "  export PATH=${executable_folder}:\$PATH"
    exit 0
}

main
