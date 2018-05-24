#!/bin/sh
#ValidateService

WORK_DIR=/opt/loopring/miner

#cron and logrotate are installed by default in ubuntu, don't check it again
if [ ! -f /etc/logrotate.d/loopring-miner ]; then
    sudo cp $WORK_DIR/src/bin/logrotate/loopring-miner /etc/logrotate.d/loopring-miner
fi

pgrep cron
if [[ $? != 0 ]]; then
    sudo /etc/init.d/cron start
fi

#check later
exit 0
