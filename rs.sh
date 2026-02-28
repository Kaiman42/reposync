#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"
service_name="reposync-watcher.service"

print_usage() {
    cat <<USAGE
Usage: rs.sh <command> [options]

Commands:
  setup        Install dependencies (inotify-tools) AND install service (configure, enable, start)
  uninstall    Uninstall service
  status       Show service status
  enable       Enable the service
  disable      Disable the service
  restart      Restart the service
  stop         Stop the service
  gui          Show visual interface
  colors       Show color legend for folder icons
  repos        Gerenciar locais monitorados (list, add, remove)


Options:
  --user       Operate on the user unit (in ~/.config/systemd/user)
  --system     Operate on the system unit (/etc/systemd/system) (requires root)

Examples:
  ./rs.sh setup
  sudo ./rs.sh setup --system
  ./rs.sh status --user
  ./rs.sh restart
USAGE
}

MODE=""
CMD=""

if [ $# -lt 1 ]; then
    print_usage
    exit 1
fi

CMD="$1"; shift || true

if [ "${1:-}" = "--user" ]; then
    MODE="user"
    shift || true
elif [ "${1:-}" = "--system" ]; then
    MODE="system"
    shift || true
fi

# Detect default mode based on uid
if [ -z "$MODE" ]; then
    if [ "$(id -u)" -eq 0 ]; then
        MODE="system"
    else
        MODE="user"
    fi
fi

# Helper to run user systemctl as correct user
run_user_systemctl() {
    if [ "$(id -u)" -eq 0 ] && [ -n "${SUDO_USER:-}" ]; then
        su - "$SUDO_USER" -c "systemctl --user $*"
    else
        systemctl --user "$@"
    fi
}

# Install dependencies
install_dependencies() {
    echo "Verificando dependências..."
    if command -v inotifywait >/dev/null 2>&1; then
        echo "Dependência 'inotify-tools' já instalada."
        return 0
    fi

    echo "Tentando instalar 'inotify-tools'..."
    if [ -x "$(command -v apt-get)" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            sudo apt-get update && sudo apt-get install -y inotify-tools
        else
            apt-get update && apt-get install -y inotify-tools
        fi
    elif [ -x "$(command -v dnf)" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            sudo dnf install -y inotify-tools
        else
            dnf install -y inotify-tools
        fi
    elif [ -x "$(command -v pacman)" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            sudo pacman -S --needed --noconfirm inotify-tools
        else
            pacman -S --needed --noconfirm inotify-tools
        fi
    elif [ -x "$(command -v zypper)" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            sudo zypper install -y inotify-tools
        else
            zypper install -y inotify-tools
        fi
    else
        echo "ERRO: Gerenciador de pacotes não suportado ou não encontrado."
        echo "Por favor, instale 'inotify-tools' manualmente."
        exit 1
    fi
}

# Install service
install_service() {
    echo "Tornando scripts executáveis..."
    chmod +x "$REPO_DIR"/bin/*.py || true

    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System install requires root. Run: sudo $0 setup --system"
            exit 1
        fi
        
        RUN_AS_USER="${SUDO_USER:-$(whoami)}"
        HOME_DIR="$(eval echo "~$RUN_AS_USER")"
        SERVICE_DST="/etc/systemd/system/$service_name"

        echo "Gerando unit systemd em: $SERVICE_DST"
        cat > "$SERVICE_DST" <<EOF
[Unit]
Description=RepoSync Watcher (inotify)
After=multi-user.target

[Service]
Type=simple
User=$RUN_AS_USER
Environment=HOME=$HOME_DIR
Environment=DEBOUNCE_MS=500
WorkingDirectory=$REPO_DIR
ExecStart=/usr/bin/env python3 $REPO_DIR/bin/watch_repos.py
Restart=always
RestartSec=5
SyslogIdentifier=reposync-watcher

[Install]
WantedBy=multi-user.target
EOF

        echo "Recarregando systemd e habilitando serviço (system)..."
        systemctl daemon-reload
        systemctl enable --now "$service_name"
        echo "Instalação concluída (system)."
    else
        # User-mode installation
        TARGET_USER="${SUDO_USER:-$(whoami)}"
        TARGET_HOME="$(eval echo "~$TARGET_USER")"
        USER_UNIT_DIR="$TARGET_HOME/.config/systemd/user"
        USER_UNIT_DST="$USER_UNIT_DIR/$service_name"

        echo "Instalando como serviço de usuário para: $TARGET_USER"
        mkdir -p "$USER_UNIT_DIR"

        cat > "$USER_UNIT_DST" <<EOF
[Unit]
Description=RepoSync Watcher (inotify)
After=graphical-session.target

[Service]
Type=simple
Environment=DEBOUNCE_MS=500
WorkingDirectory=$REPO_DIR
ExecStart=/usr/bin/env python3 $REPO_DIR/bin/watch_repos.py
Restart=always
RestartSec=5
SyslogIdentifier=reposync-watcher

[Install]
WantedBy=default.target
EOF

        chown "$TARGET_USER":"$TARGET_USER" "$USER_UNIT_DST" || true

        echo "Recarregando daemon do systemd (user) e habilitando serviço..."

        if [ "$(id -u)" -eq 0 ] && [ "$TARGET_USER" != "root" ]; then
            su - "$TARGET_USER" -c "systemctl --user daemon-reload >/dev/null 2>&1 || true; systemctl --user enable --now $service_name"
        else
            systemctl --user daemon-reload >/dev/null 2>&1 || true
            systemctl --user enable --now "$service_name"
        fi

        echo "Instalação concluída (user)."
    fi
}

# Uninstall service
uninstall_service() {
    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System uninstall requires root. Run: sudo $0 uninstall --system"
            exit 1
        fi
        systemctl stop "$service_name" || true
        systemctl disable "$service_name" || true
        rm -f "/etc/systemd/system/$service_name" || true
        systemctl daemon-reload
        echo "System service removed."
    else
        # User uninstall
        if [ "$(id -u)" -eq 0 ] && [ -n "${SUDO_USER:-}" ]; then
            TARGET_HOME=$(eval echo "~$SUDO_USER")
            TARGET_UNIT=$TARGET_HOME/.config/systemd/user/$service_name
            su - "$SUDO_USER" -c "systemctl --user stop $service_name || true; systemctl --user disable $service_name || true" || true
            rm -f "$TARGET_UNIT" || true
            su - "$SUDO_USER" -c "systemctl --user daemon-reload" || true
            echo "User service removed for $SUDO_USER."
        else
            run_user_systemctl stop "$service_name" || true
            run_user_systemctl disable "$service_name" || true
            rm -f "$HOME/.config/systemd/user/$service_name" || true
            systemctl --user daemon-reload || true
            echo "User service removed."
        fi
    fi
}

# Show status
show_status() {
    if [ "$MODE" = "system" ]; then
        systemctl status "$service_name" || true
    else
        run_user_systemctl status "$service_name" || true
    fi
}

# Enable service
enable_service() {
    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System enable requires root. Run: sudo $0 enable --system"
            exit 1
        fi
        systemctl enable --now "$service_name"
    else
        run_user_systemctl enable --now "$service_name"
    fi
}

# Disable service
disable_service() {
    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System disable requires root. Run: sudo $0 disable --system"
            exit 1
        fi
        systemctl disable --now "$service_name"
    else
        run_user_systemctl disable --now "$service_name"
    fi
}

# Restart service
restart_service() {
    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System restart requires root. Run: sudo $0 restart --system"
            exit 1
        fi
        systemctl restart "$service_name"
    else
        run_user_systemctl restart "$service_name"
    fi
}

# Stop service
stop_service() {
    if [ "$MODE" = "system" ]; then
        if [ "$(id -u)" -ne 0 ]; then
            echo "System stop requires root. Run: sudo $0 stop --system"
            exit 1
        fi
        systemctl stop "$service_name"
    else
        run_user_systemctl stop "$service_name"
    fi
}

# Show Colors Legend
show_colors() {
    echo "=== Legenda de Cores (Ícones de Pasta) ==="
    echo -e "\033[1;32m● Verde\033[0m    : Sincronizado (Clean)"
    echo -e "\033[1;33m● Amarelo\033[0m  : Alterado (Mudanças não commitadas - Modified/Deleted)"
    echo -e "\033[1;31m● Vermelho\033[0m : Não Rasteado (Novos arquivos - Untracked)"
    echo -e "\033[1;35m● Violeta\033[0m  : Pendente (Sync necessário - Ahead/Behind)"
    # Tentar usar laranja via 256 colors (208), fallback para amarelo escuro se não suportado, mas na dúvida usamos o texto claro
    echo -e "\033[38;5;208m● Laranja\033[0m  : Sem Remote (Repositório apenas local)"
    echo -e "\033[1;30m● Preto\033[0m    : Não Git / Não inicializado / Erro"
    echo "=========================================="
}

# Show GUI
show_gui() {
    local GUI_TOOL=""
    if command -v zenity >/dev/null 2>&1; then
        GUI_TOOL="zenity"
    elif command -v whiptail >/dev/null 2>&1; then
        GUI_TOOL="whiptail"
    else
        echo "Erro: Nem 'zenity' nem 'whiptail' encontrados para interface visual."
        echo "Instale um deles para usar o modo GUI."
        exit 1
    fi

    local ACTION=""
    
    while true; do
        if [ "$GUI_TOOL" = "zenity" ]; then
            ACTION=$(zenity --list --title="RepoSync Manager ($MODE)" \
                --text="Escolha uma ação:" \
                --column="Comando" --column="Descrição" \
                "status" "Ver status do serviço" \
                "restart" "Reiniciar serviço" \
                "stop" "Parar serviço" \
                "colors" "Ver legenda de cores" \
                "enable" "Habilitar e Iniciar serviço" \
                "disable" "Parar e Desabilitar serviço" \
                "setup" "Instalar/Configurar dependências e serviço" \
                "uninstall" "Remover serviço completamente" \
                --height=400 --width=500 --print-column=1 2>/dev/null) || break
        else
            ACTION=$(whiptail --title "RepoSync Manager ($MODE)" --menu "Escolha uma ação:" 20 70 10 \
                "status" "Ver status do serviço" \
                "restart" "Reiniciar serviço" \
                "stop" "Parar serviço" \
                "colors" "Ver legenda de cores" \
                "enable" "Habilitar e Iniciar serviço" \
                "disable" "Parar e Desabilitar serviço" \
                "setup" "Instalar/Configurar dependências e serviço" \
                "uninstall" "Remover serviço completamente" 3>&1 1>&2 2>&3) || break
        fi

        [ -z "$ACTION" ] && break

        # Helper to capture execution output and show it
        local OUTPUT
        if OUTPUT=$(bash "$REPO_DIR/rs.sh" "$ACTION" "--$MODE" 2>&1); then
            if [ "$GUI_TOOL" = "zenity" ]; then
                echo "$OUTPUT" | zenity --text-info --title="Resultado: $ACTION" --width=700 --height=500
            else
                echo "$OUTPUT" | whiptail --title "Resultado: $ACTION" --scrolltext --textbox /dev/stdin 20 70
            fi
        else
            if [ "$GUI_TOOL" = "zenity" ]; then
                echo "$OUTPUT" | zenity --text-info --title="Erro: $ACTION" --width=700 --height=500 --ok-label="Fechar"
            else
                echo "$OUTPUT" | whiptail --title "Erro: $ACTION" --scrolltext --textbox /dev/stdin 20 70
            fi
        fi
    done
}

# Manage repositories/paths
manage_repos() {
    local SUB_CMD="${1:-list}"
    local ARG="${2:-}"
    local CONFIG="$REPO_DIR/watched_paths.conf"

    # Inicializa com defaults se não existir
    if [ ! -f "$CONFIG" ]; then
        echo "/home/kaiman/Repos/Github/Meus" > "$CONFIG"
        echo "/home/kaiman/Repos/Github/Terceiros" >> "$CONFIG"
    fi

    case "$SUB_CMD" in
        list)
            echo "=== Locais Monitorados ==="
            if [ -s "$CONFIG" ]; then
                cat -n "$CONFIG"
            else
                echo "(Lista vazia)"
            fi
            echo "=========================="
            ;;
        add)
            if [ -z "$ARG" ]; then
                echo "Uso: $0 repos add <caminho>"
                exit 1
            fi
            
            # Resolve path absolute
            local ABS_PATH
            if ! ABS_PATH=$(readlink -f "$ARG" 2>/dev/null); then
                echo "Erro: Caminho inválido."
                exit 1
            fi

            if [ ! -d "$ABS_PATH" ]; then
                echo "Erro: Diretório não encontrado: $ABS_PATH"
                exit 1
            fi
            
            if grep -Fxq "$ABS_PATH" "$CONFIG"; then
                echo "Aviso: Já está na lista: $ABS_PATH"
            else
                echo "$ABS_PATH" >> "$CONFIG"
                echo "Adicionado: $ABS_PATH"
                echo "Nota: Execute '$0 restart' para aplicar."
            fi
            ;;
        remove)
            if [ -z "$ARG" ]; then
                echo "Uso: $0 repos remove <caminho>"
                exit 1
            fi

            local ABS_PATH
            # Tenta resolver, se falhar usa o argumento cru (caso o dir tenha sido deletado)
            ABS_PATH=$(readlink -f "$ARG" 2>/dev/null || echo "$ARG")
             
            if grep -Fq "$ABS_PATH" "$CONFIG"; then
                grep -Fv "$ABS_PATH" "$CONFIG" > "${CONFIG}.tmp" && mv "${CONFIG}.tmp" "$CONFIG"
                echo "Removido: $ABS_PATH"
                echo "Nota: Execute '$0 restart' para aplicar."
            else
                echo "Erro: Caminho não encontrado na lista: $ABS_PATH"
            fi
            ;;
        *)
            echo "Comando inválido. Uso: $0 repos <list|add|remove>"
            exit 1
            ;;
    esac
}

# Main dispatch
case "$CMD" in
    setup)
        install_dependencies
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    status)
        show_status
        ;;
    enable)
        enable_service
        ;;
    disable)
        disable_service
        ;;
    restart)
        restart_service
        ;;
    stop)
        stop_service
        ;;
    colors)
        show_colors
        ;;
    gui)
        show_gui
        ;;
    repos)
        # Shift CMD
        SUB_CMD="${1:-}"
        ARGS="${2:-}"
        # Se usarmos shift no topo, $1 agora é o SUB_CMD
        # Mas no bash script original: CMD="$1"; shift
        # Então $1 é o primeiro argumento APÓS 'repos'
        manage_repos "${1:-}" "${2:-}"
        ;;
    *)
        print_usage
        exit 1
        ;;
esac

exit 0
