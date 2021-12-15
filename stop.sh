#!/bin/bash
pid=`ps -elf | grep "serverMonitorRobot_linux" | grep -v "grep" | awk '{print $4}'`
if [ -z "$pid" ]; then
        echo "no process to stop"
        exit 1
fi

#echo "stop success"
`kill -TERM $pid`
