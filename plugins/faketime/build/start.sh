#!/bin/bash

get_child_pids() {
    local parent_pid=$1
    local child_pids_list=$(pgrep -P $parent_pid)

    for child_pid in $child_pids_list; do
        child_pids+=("$child_pid")
        get_child_pids "$child_pid"
    done
}

declare -a child_pids=()
process_array=(`echo $modify_process_name | tr ',' ' '`)

for process_name in ${process_array[@]}
do
  sp_pid=`pgrep -x $process_name`
  if [ -n "$sp_pid" ]
  then
    child_pids+=("$sp_pid")
    if [ "$Modify_Sub_Process" == "true" ]
    then
      get_child_pids "$sp_pid"
    fi
  fi
done


echo "List of processes that will be modified： ${child_pids[*]}"
for modify_process_pid in ${child_pids[@]}
do
  echo "start modify process pid: ${modify_process_pid}"
  ./bin/watchmaker -pid $modify_process_pid -sec_delta $delay_second -nsec_delta $delay_nanosecond -clk_ids "CLOCK_REALTIME,CLOCK_MONOTONIC"
done