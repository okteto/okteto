#!/usr/bin/env bash
set -euo pipefail

# =======================
# Configuration Variables
# =======================
GITHUB_ORG="okteto"
DOCKER_IMAGE="okteto/okteto"
LOG_FILE="./rollback.log"

# =======================
# Initialization
# =======================
DRY_RUN=false
VERBOSE=false

# List of known actions
ACTIONS=(
    test
    pipeline
    namespace
    destroy-preview
    destroy-pipeline
    deploy-preview
    delete-namespace
    create-namespace
    context
    build
)

# =======================
# Logging Functions
# =======================

# -----------------------
# log: General logging function.
#
# Description:
#   Logs messages with a specified log level and appends them to the log file.
#
# Usage Example:
#   log "INFO" "This is an informational message."
#
log() {
    local LEVEL="$1"
    local MESSAGE="$2"
    echo "$(date +"%Y-%m-%d %H:%M:%S") [$LEVEL] $MESSAGE" | tee -a "$LOG_FILE"
}

# -----------------------
# info: Logs informational messages.
#
# Description:
#   Utilizes the log function to log messages at the INFO level.
#
# Usage Example:
#   info "Starting the rollback process."
#
info() {
    log "INFO" "$1"
}

# -----------------------
# warn: Logs warning messages.
#
# Description:
#   Utilizes the log function to log messages at the WARNING level.
#
# Usage Example:
#   warn "The rollback tag does not exist. Skipping repository."
#
warn() {
    log "WARNING" "$1"
}

# -----------------------
# error: Logs error messages and directs them to stderr.
#
# Description:
#   Utilizes the log function to log messages at the ERROR level and ensures they are sent to the standard error stream.
#
# Usage Example:
#   error "Failed to clone the repository."
#
error() {
    log "ERROR" "$1" >&2
}

# -----------------------
# usage: Displays the help and usage information for the script.
#
# Description:
#   Provides detailed instructions on how to use the script, including available options and arguments.
#
# Usage Example:
#   usage
#
usage() {
    cat <<EOF
Usage: $0 [OPTIONS] <tag_to_move> <rollback_to_tag>

Options:
  --dry-run        Perform a trial run without making any changes.
  --verbose        Enable verbose output.
  --help           Display this help message.

Arguments:
  tag_to_move      The tag you want to move/update.
  rollback_to_tag  The tag to which you want to rollback.

Example:
  $0 --dry-run 1.2.3 1.2.2
EOF
}

# -----------------------
# parse_args: Parses and validates command-line arguments.
#
# Description:
#   Processes global options like --dry-run and --verbose, and ensures that the required positional arguments are provided.
#
# Usage Example:
#   parse_args "$@"
#
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help)
                usage
                exit 0
                ;;
            *)
                break
                ;;
        esac
    done

    if [ $# -ne 2 ]; then
        error "Invalid number of arguments."
        usage
        exit 1
    fi

    TAG_TO_MOVE="$1"
    ROLLBACK_TO_TAG="$2"
}

# -----------------------
# check_dependencies: Ensures required commands are available.
#
# Description:
#   Verifies that necessary tools like git and docker are installed before proceeding.
#
# Usage Example:
#   check_dependencies
#
check_dependencies() {
    local dependencies=(git docker)
    for cmd in "${dependencies[@]}"; do
        if ! command -v "$cmd" &>/dev/null; then
            error "Required command '$cmd' is not installed. Aborting."
            exit 1
        fi
    done
}

# -----------------------
# process_repository: Handles the rollback for a single repository.
#
# Description:
#   Clones the specified repository, moves the Git tag to the rollback tag, pushes the changes, and cleans up the temporary directory.
#
# Usage Example:
#   process_repository "pipeline" "1.2.3" "v1.2.2"
#
process_repository() {
    local action="$1"
    local tag_move="$2"
    local tag_rollback="$3"
    local temp_dir
    temp_dir=$(mktemp -d)

    info "Processing action: $action"

    git clone "https://github.com/${GITHUB_ORG}/${action}.git" "$temp_dir/$action" &>> "$LOG_FILE"

    pushd "$temp_dir/$action" > /dev/null

    git checkout main &>> "$LOG_FILE"
    git fetch --all --tags &>> "$LOG_FILE"

    # Check if the rollback target exists
    if ! git rev-parse "$tag_rollback" >/dev/null 2>&1; then
        warn "Tag '$tag_rollback' does not exist in repository '$action'. Skipping..."
        popd > /dev/null
        rm -rf "$temp_dir"
        return
    fi

    # Check if the tag already points to the rollback commit
    current_commit=$(git rev-parse "$tag_move")
    rollback_commit=$(git rev-parse "$tag_rollback")
    if [ "$current_commit" == "$rollback_commit" ]; then
        info "Tag '$tag_move' in '$action' is already pointing to '$tag_rollback'. Skipping..."
        popd > /dev/null
        rm -rf "$temp_dir"
        return
    fi

    # Move the tag
    if [ "$DRY_RUN" = true ]; then
        info "[DRY RUN] Would run: git tag -f '$tag_move' '$tag_rollback'"
        info "[DRY RUN] Would run: git push origin -f '$tag_move'"
    else
        git tag -f "$tag_move" "$tag_rollback" &>> "$LOG_FILE"
        git push origin -f "$tag_move" &>> "$LOG_FILE"
        info "Tag '$tag_move' has been successfully rolled back to '$tag_rollback' in repository '$action'."
    fi

    popd > /dev/null
    rm -rf "$temp_dir"
}

# -----------------------
# rollback_docker_image: Rolls back the Docker image to a specified tag.
#
# Description:
#   Pulls the Docker image corresponding to the rollback tag, tags it as latest, and pushes it to the Docker registry.
#
# Usage Example:
#   rollback_docker_image "okteto/okteto" "1.2.2"
#
rollback_docker_image() {
    local image="$1"
    local tag="$2"

    info "Starting Docker image rollback for $image:latest and stable to tag '$tag'."

    if [ "$DRY_RUN" = true ]; then
        info "[DRY RUN] Would pull $image:$tag"
        info "[DRY RUN] Would tag $image:$tag as $image:latest"
        info "[DRY RUN] Would tag $image:$tag as $image:stable"
        info "[DRY RUN] Would push $image:latest to the Docker registry"
        info "[DRY RUN] Would push $image:stable to the Docker registry"
    else
        info "Pulling $image:$tag..."
        docker pull "${image}:${tag}" &>> "$LOG_FILE"

        info "Tagging $image:$tag as $image:latest..."
        docker tag "${image}:${tag}" "${image}:latest" &>> "$LOG_FILE"

        info "Tagging $image:$tag as $image:stable..."
        docker tag "${image}:${tag}" "${image}:stable" &>> "$LOG_FILE"

        info "Pushing $image:latest to the Docker registry..."
        docker push "${image}:latest" &>> "$LOG_FILE"

        info "Pushing $image:stable to the Docker registry..."
        docker push "${image}:stable" &>> "$LOG_FILE"

        info "Docker image $image:latest and $image:stable has been successfully rolled back to '$tag'."
    fi
}


# -----------------------
# main: Orchestrates the rollback process.
#
# Description:
#   Parses arguments, checks dependencies, processes each repository, and performs the Docker image rollback.
#
# Usage Example:
#   main "$@"
#
main() {
    parse_args "$@"
    check_dependencies

    if [ "$VERBOSE" = true ]; then
        set -x
    fi

    info "Starting rollback process."
    info "Tag to move: '$TAG_TO_MOVE'"
    info "Rollback to tag: '$ROLLBACK_TO_TAG'"
    [ "$DRY_RUN" = true ] && info "Dry run mode enabled."

    # Process each repository
    for action in "${ACTIONS[@]}"; do
        process_repository "$action" "$TAG_TO_MOVE" "$ROLLBACK_TO_TAG"
    done

    # Docker Image Rollback
    rollback_docker_image "$DOCKER_IMAGE" "$ROLLBACK_TO_TAG"

    info "Rollback process completed successfully."
}

# -----------------------
# Trap: Handles unexpected errors.
#
# Description:
#   Catches any errors during script execution and logs an appropriate error message before exiting.
#
# Usage Example:
#   Automatically handled by the script.
#
trap 'error "An unexpected error occurred. Exiting."; exit 1' ERR

# -----------------------
# Execution Entry Point
#
# Description:
#   Initiates the main function with all passed command-line arguments.
#
# Usage Example:
#   Automatically handled by the script.
#
main "$@"
