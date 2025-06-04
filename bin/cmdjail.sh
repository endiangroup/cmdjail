#!/usr/bin/env bash
#version=v0.1.2

DE=${DEBUG:-""}
CMDJAIL_FILENAME=".cmd.jail"
DIR=$(dirname "$0")
SCRIPT=$(basename "$0")
LOG=${CMDJAIL_LOG:-""}

_debug() {
  local message="$1"
  [ ${DE} ] && echo "[debug] ${message}"
}

_debug "\$@=$@"
_debug "\$CMDJAIL_FILENAME=$CMDJAIL_FILENAME"
_debug "\$DIR=$DIR"
_debug "\$LOG=$LOG"
_debug "\$SCRIPT=$SCRIPT"

_log_message() {
  local level="$1"
  local message="$2"
  if [ -n "${LOG}" ]; then
    echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") -- [${level}]: ${message}" >> "${LOG}"
  fi
}

_error() {
  local message="$1"
  >&2 echo -e "[error]: ${message}"
  _log_message "error" "${message}"
}

if [ ! -f ${CMDJAIL_FILENAME} -a ! -f ${DIR}/${CMDJAIL_FILENAME} ]; then
  _error "no .cmd.jail file found\n\tsearched:\n\t\t$(pwd)\n\t\t$(pwd)/$(basename ${DIR})"
  exit 126
fi

# Force IFS character
IFS=" "
ARGS_AFTER_DOUBLE_DASH=()
# Use everything after the last `--` to set the $INTENDED_CMD
LAST_DOUBLE_DASH_INDEX=-1
for ((i=${#@}; i>=1; i--)); do
    if [ "${!i}" = "--" ]; then
        LAST_DOUBLE_DASH_INDEX=$i
        break
    fi
done

if [ $LAST_DOUBLE_DASH_INDEX -ne -1 ]; then
    for ((i=LAST_DOUBLE_DASH_INDEX+1; i<=${#@}; i++)); do
        ARGS_AFTER_DOUBLE_DASH+=("${!i}")
    done
fi
_debug "\$ARGS_AFTER_DOUBLE_DASH=${ARGS_AFTER_DOUBLE_DASH[@]}"

INTENDED_CMD=(${ARGS_AFTER_DOUBLE_DASH[@]})

# Use an environment varialbe that references another environment variable
# to set the $INTENDED_CMD
if [ ! -z "$CMDJAIL_ENV_REFERENCE" ]; then
  _debug "\$CMDJAIL_ENV_REFERENCE=$CMDJAIL_ENV_REFERENCE"
  if [ ! -z "${!CMDJAIL_ENV_REFERENCE}" ]; then
    _debug "\$${!CMDJAIL_ENV_REFERENCE}=${!CMDJAIL_ENV_REFERENCE}"
    INTENDED_CMD=(${!CMDJAIL_ENV_REFERENCE})
  fi
fi

# Use an environment variable to set the $INTENDED_CMD
if [ ! -z "$CMDJAIL_CMD" ]; then
  _debug "\$CMDJAIL_CMD=$CMDJAIL_CMD"
  INTENDED_CMD=(${CMDJAIL_CMD})
fi

_debug "\$INTENDED_CMD=$INTENDED_CMD"

if [ -z "${INTENDED_CMD[0]}" ]; then
  _error "no command"
  exit 126
fi
if echo "${INTENDED_CMD[@]}" | grep -q "${CMDJAIL_FILENAME}"; then
  _error "attempting to manipulate ${CMDJAIL_FILENAME}. Aborting."
  exit 126
fi
if echo "${INTENDED_CMD[@]}" | grep -q "${SCRIPT}"; then
  _error "attempting to manipulate ${SCRIPT}. Aborting."
  exit 126
fi

WHITELIST=$([ -f ${CMDJAIL_FILENAME} ] && echo ${CMDJAIL_FILENAME} || echo ${DIR}/${CMDJAIL_FILENAME})
_debug "found $CMDJAIL_FILENAME file"
_debug "\$WHITELIST=$WHITELIST"
_debug "$CMDJAIL_FILENAME:\n$(cat $WHITELIST)"

if cat $WHITELIST | grep -q ${INTENDED_CMD[0]}; then
  _debug "running: ${INTENDED_CMD[@]}"
  ${INTENDED_CMD[@]}
else
  _log_message "warn" "blocked blacklisted command: ${INTENDED_CMD[*]}"
  exit 2
fi
