#!/bin/bash

array=(`echo $modify_process_name | tr ',' ' '`)
for modify_process in ${array[@]}
do
  sp_pid=`ps ax|grep $modify_process|grep -v grep|grep -v /bin/sh|awk '{print $1}'`
  if [ -n "$sp_pid" ]
  then
    ./bin/watchmaker -pid $sp_pid -sec_delta $delay_second -nsec_delta 0 -clk_ids "CLOCK_REALTIME,CLOCK_MONOTONIC"
  fi
done
