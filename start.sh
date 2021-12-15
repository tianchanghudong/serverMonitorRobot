#!/bin/bash

ulimit -c unlimited
chmod +x serverMonitorRobot_linux
env GOTRACEBACK=crash ./serverMonitorRobot_linux > output &
