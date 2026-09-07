#!/bin/sh
# Explicit phases, no destructive shortcuts. Run from repository root.
set -eu
ROOT=infrastructure/kubernetes
action=${1:?render|apply|job}
topology=${2:?single-node|ha}
phase=${3:?apps|database|configuration|pipeline|api|web|ingress|migrations|bootstrap|analytics-refresh|models-refresh}
case "$topology" in single-node|ha) ;; *) exit 2 ;; esac
case "$phase" in
  database) path="$ROOT/database/overlays/$topology" ;;
  apps|configuration|pipeline|api|web|ingress) path="$ROOT/apps/overlays/$topology" ;;
  migrations) path="$ROOT/migrations" ;;
  bootstrap|analytics-refresh|models-refresh) path="$ROOT/maintenance/$phase" ;;
  *) echo "Unknown phase" >&2; exit 2 ;;
esac
if [ "$action" != render ]; then
  : "${KUBE_CONTEXT:?Set KUBE_CONTEXT to the explicit production context}"
  : "${IMAGE_TAG:?Set IMAGE_TAG=sha- followed by the full 40-character release SHA}"
  : "${PUBLIC_HOST:?Set the public DNS hostname}"
  : "${ACME_EMAIL:?Set the certificate contact}"
  printf '%s' "$IMAGE_TAG" | grep -Eq '^sha-[0-9a-f]{40}$'
  printf '%s' "$PUBLIC_HOST" | grep -Eq '^[a-z0-9][a-z0-9.-]+[a-z0-9]$'
  case "$PUBLIC_HOST" in *.invalid|*example.com) echo "Replace example hostname" >&2; exit 2 ;; esac
  if [ "$phase" = database ]; then
    : "${BACKUP_BUCKET:?Set a private off-cluster S3 bucket}"
    : "${BACKUP_ENDPOINT:?Set the HTTPS S3 endpoint}"
  fi
fi
# Limit substitution to known non-secret placeholders. Reject sed metacharacters.
for value in "${PUBLIC_HOST:-}" "${ACME_EMAIL:-}" "${BACKUP_BUCKET:-}" "${BACKUP_ENDPOINT:-}"; do
  case "$value" in *'|'*|*'&'*|*'\'* ) echo "Unsupported configuration character" >&2; exit 2 ;; esac
done
# Fail before the apply pipeline if Kustomize fails (POSIX sh has no pipefail).
rendered=$(kubectl kustomize "$path")
render() {
  printf '%s\n' "$rendered" | sed \
    -e "s|sha-RELEASE_SHA|${IMAGE_TAG:-sha-RELEASE_SHA}|g" \
    -e "s|app.example.invalid|${PUBLIC_HOST:-app.example.invalid}|g" \
    -e "s|operator@example.invalid|${ACME_EMAIL:-operator@example.invalid}|g" \
    -e "s|REPLACE_BACKUP_BUCKET|${BACKUP_BUCKET:-REPLACE_BACKUP_BUCKET}|g" \
    -e "s|https://s3.example.invalid|${BACKUP_ENDPOINT:-https://s3.example.invalid}|g"
}
case "$action" in
  render) render ;;
  apply)
    case "$phase" in
      apps|database) render | kubectl --context "$KUBE_CONTEXT" apply -f - ;;
      configuration|pipeline|api|web|ingress)
        render | kubectl --context "$KUBE_CONTEXT" apply -f - -l "app.kubernetes.io/component=$phase" ;;
      *) echo "Use job for one-shot operations" >&2; exit 2 ;;
    esac ;;
  job)
    case "$phase" in migrations|bootstrap|analytics-refresh|models-refresh) ;; *) exit 2 ;; esac
    job_run_id=${JOB_RUN_ID:-$(date -u +%Y%m%d%H%M%S)}
    printf '%s' "$job_run_id" | grep -Eq '^[a-z0-9-]+$'
    render | sed "s/-manual$/-$job_run_id/" | kubectl --context "$KUBE_CONTEXT" create -f - -o name ;;
  *) exit 2 ;;
esac
