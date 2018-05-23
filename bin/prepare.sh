#!/bin/bash
#BeforeInstall
WORK_DIR=/opt/loopring/miner

if [ ! -d $WORK_DIR/src ]; then
	mkdir -p $WORK_DIR/src
	chown -R ubuntu:ubuntu $WORK_DIR
fi

if [ ! -d $WORK_DIR/bin ]; then
	mkdir -p $WORK_DIR/bin
	chown -R ubuntu:ubuntu $WORK_DIR
fi

which go
if [[ $? != 0 ]]; then
	echo "golang not installed, begin install !!!"
	apt-get update
	apt install golang-1.9-go -y
fi

SVC_DIR=/etc/service/miner

if [ ! -d $SVC_DIR ]; then
       mkdir -p $SVC_DIR
fi

#stop former service
svc -d $SVC_DIR

# clear work dir
rm -rf $WORK_DIR/src/*
rm -rf $WORK_DIR/src/.[a-z]*
rm -rf $WORK_DIR/bin/*

#cron and logrotate are installed by default in ubuntu, don't check it again
if [ ! -f /etc/logrotate.d/loopring-miner ]; then
    sudo cp $WORK_DIR/src/bin/logrotate/loopring-miner /etc/logrotate.d/loopring-miner
fi

pgrep cron
if [[ $? != 0 ]]; then
    sudo /etc/init.d/cron start
fi

