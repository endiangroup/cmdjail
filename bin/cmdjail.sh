#!/usr/bin/env bash
#version=v0.1.2

DE=${DEBUG:-""}
CMDJAIL_FILENAME=".cmd.jail"
DIR=$(dirname "$0")
SCRIPT=$(basename "$0")
LOG=${CMDJAIL_LOG:-""}

[ ${DE} ] && echo "[debug] \$@=$@"
[ ${DE} ] && echo "[debug] \$CMDJAIL_FILENAME=$CMDJAIL_FILENAME"
[ ${DE} ] && echo "[debug] \$DIR=$DIR"
[ ${DE} ] && echo "[debug] \$LOG=$LOG"
[ ${DE} ] && echo "[debug] \$SCRIPT=$SCRIPT"

if [ ! -f ${CMDJAIL_FILENAME} -a ! -f ${DIR}/${CMDJAIL_FILENAME} ]; then
  >&2 echo -e "[error]: no .cmd.jail file found\n\tsearched:\n\t\t$(pwd)\n\t\t$(pwd)/$(basename ${DIR})"
  exit 126
fi

FOUND_DOUBLE_DASH=false
ARGS_AFTER_DOUBLE_DASH=()
# Use everything after a `--` to set the $INTENDED_CMD
for arg in "$@"; do
    if [ "$FOUND_DOUBLE_DASH" = true ]; then
        ARGS_AFTER_DOUBLE_DASH+=("$arg")
    elif [ "$arg" = "--" ]; then
        FOUND_DOUBLE_DASH=true
    fi
done
[ ${DE} ] && echo "[debug] \$ARGS_AFTER_DOUBLE_DASH=${ARGS_AFTER_DOUBLE_DASH[@]}"

INTENDED_CMD=(${ARGS_AFTER_DOUBLE_DASH[@]})

# Use an environment varialbe that references another environment variable
# to set the $INTENDED_CMD
if [ ! -z "$CMDJAIL_ENV_REFERENCE" ]; then
  [ ${DE} ] && echo "[debug] \$CMDJAIL_ENV_REFERENCE=$CMDJAIL_ENV_REFERENCE"
  if [ ! -z "${!CMDJAIL_ENV_REFERENCE}" ]; then
    [ ${DE} ] && echo "[debug] \$${!CMDJAIL_ENV_REFERENCE}=${!CMDJAIL_ENV_REFERENCE}"
    INTENDED_CMD=(${!CMDJAIL_ENV_REFERENCE})
  fi
fi

# Use an environment variable to set the $INTENDED_CMD
if [ ! -z "$CMDJAIL_CMD" ]; then
  [ ${DE} ] && echo "[debug] \$CMDJAIL_CMD=$CMDJAIL_CMD"
  INTENDED_CMD=(${CMDJAIL_CMD})
fi

[ ${DE} ] && echo "[debug] \$INTENDED_CMD=$INTENDED_CMD"

if [ -z "${INTENDED_CMD[0]}" ]; then
  >&2 echo -e "[error]: no command"
  [ ${LOG} ] && echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") -- [error]: no command" >> $LOG
  exit 126
fi
if echo "${INTENDED_CMD[@]}" | grep -q "${CMDJAIL_FILENAME}"; then
  >&2 echo -e "[error]: attempting to manipulate ${CMDJAIL_FILENAME}. Aborting."
  [ ${LOG} ] && echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") -- [error]: attempting to manipulate ${CMDJAIL_FILENAME}. Aborting." >> $LOG
  exit 126
fi
if echo "${INTENDED_CMD[@]}" | grep -q "${SCRIPT}"; then
  >&2 echo -e "[error]: attempting to manipulate ${SCRIPT}. Aborting."
  [ ${LOG} ] && echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") -- [error]: attempting to manipulate ${SCRIPT}. Aborting." >> $LOG
  exit 126
fi

WHITELIST=$([ -f ${CMDJAIL_FILENAME} ] && echo ${CMDJAIL_FILENAME} || echo ${DIR}/${CMDJAIL_FILENAME})
[ ${DE} ] && echo "[debug] found $CMDJAIL_FILENAME file"
[ ${DE} ] && echo "[debug] \$WHITELIST=$WHITELIST"
[ ${DE} ] && echo -e "[debug] $CMDJAIL_FILENAME:\n$(cat $WHITELIST)"

if cat $WHITELIST | grep -q ${INTENDED_CMD[0]}; then
  [ ${DE} ] && echo "[debug] running: ${INTENDED_CMD[@]}"
  ${INTENDED_CMD[@]}
else
  [ ${LOG} ] && echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") -- [warn] blocked blacklisted command: ${INTENDED_CMD[@]}" >> $LOG
  exit 2
fi
