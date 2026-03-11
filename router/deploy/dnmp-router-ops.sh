#!/usr/bin/env bash
set -euo pipefail

DNMP_DIR="/dnmp"
CONF_DIR="${DNMP_DIR}/services/nginx/conf.d"
ROUTER_CONF_SRC="${DNMP_DIR}/services/nginx/conf.d/starai-router.conf"
TS="$(date +%Y%m%d_%H%M%S)"

usage() {
  cat <<'EOF'
Usage:
  dnmp-router-ops.sh backup
  dnmp-router-ops.sh restore <backup_dir>
  dnmp-router-ops.sh reload
  dnmp-router-ops.sh renew-cert

Commands:
  backup      Backup current nginx conf.d to /dnmp/services/nginx/conf.d.backup.<timestamp>
  restore     Restore nginx conf.d from backup dir, then reload nginx container
  reload      Validate and reload dnmp nginx container
  renew-cert  Re-issue cert (star-ai.net/www/api) via acme.sh, install to dnmp ssl path, reload nginx
EOF
}

backup_conf() {
  local backup_dir="${DNMP_DIR}/services/nginx/conf.d.backup.${TS}"
  cp -a "${CONF_DIR}" "${backup_dir}"
  echo "backup_dir=${backup_dir}"
}

reload_nginx() {
  (cd "${DNMP_DIR}" && docker compose up -d nginx >/dev/null)
  docker exec nginx nginx -t
  docker exec nginx nginx -s reload
  docker ps --filter name=^/nginx$ --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
}

restore_conf() {
  local src="${1:-}"
  if [[ -z "${src}" || ! -d "${src}" ]]; then
    echo "restore requires valid backup_dir"
    exit 1
  fi
  rm -rf "${CONF_DIR}"
  cp -a "${src}" "${CONF_DIR}"
  reload_nginx
}

renew_cert() {
  (cd "${DNMP_DIR}" && docker compose stop nginx >/dev/null || true)

  ~/.acme.sh/acme.sh --issue --standalone \
    -d star-ai.net -d www.star-ai.net -d api.star-ai.net \
    --server letsencrypt --force

  install -m 644 /root/.acme.sh/star-ai.net/fullchain.cer /dnmp/services/nginx/ssl/starai/web/fullchain.pem
  install -m 600 /root/.acme.sh/star-ai.net/star-ai.net.key /dnmp/services/nginx/ssl/starai/web/privkey.pem
  install -m 644 /root/.acme.sh/star-ai.net/fullchain.cer /dnmp/services/nginx/ssl/starai/go/fullchain.pem
  install -m 600 /root/.acme.sh/star-ai.net/star-ai.net.key /dnmp/services/nginx/ssl/starai/go/privkey.pem

  reload_nginx
  echo "cert_renew_done"
}

cmd="${1:-}"
case "${cmd}" in
  backup)
    backup_conf
    ;;
  restore)
    restore_conf "${2:-}"
    ;;
  reload)
    reload_nginx
    ;;
  renew-cert)
    renew_cert
    ;;
  *)
    usage
    exit 1
    ;;
esac
