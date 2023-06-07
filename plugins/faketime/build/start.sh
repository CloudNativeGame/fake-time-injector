#! /bin/bash
#sleep 2
#sp_pid=`ps ax|grep $modify_process_name|grep -v grep|grep -v /bin/sh|awk '{print $1}'`
#./bin/watchmaker -pid $sp_pid -sec_delta $delay_second -nsec_delta 0 -clk_ids "CLOCK_REALTIME,CLOCK_MONOTONIC"
array=(`echo $modify_process_name | tr ',' ' '`)
for modify_process in ${array[@]}
do
  sp_pid=`ps ax|grep $modify_process|grep -v grep|grep -v /bin/sh|awk '{print $1}'`
  ./bin/watchmaker -pid $sp_pid -sec_delta $delay_second -nsec_delta 0 -clk_ids "CLOCK_REALTIME,CLOCK_MONOTONIC"
done