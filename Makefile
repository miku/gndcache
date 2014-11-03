SHELL := /bin/bash
TARGETS = gndcache

# http://docs.travis-ci.com/user/languages/go/#Default-Test-Script
test:
	go get -d && go test -v

fmt:
	go fmt ./...

imports:
	goimports -w .

all: fmt test
	go build

install:
	go install

clean:
	go clean
	rm -fv coverage.out
	rm -fv gndcache
	rm -fv *.x86_64.rpm
	rm -fv debian/gndcache*.deb
	rm -rfv debian/gndcache/usr

cover:
	go get -d && go test -v	-coverprofile=coverage.out
	go tool cover -html=coverage.out

gndcache:
	go build cmd/gndcache/gndcache.go

# experimental deb building
deb: $(TARGETS)
	mkdir -p debian/gndcache/usr/sbin
	cp $(TARGETS) debian/gndcache/usr/sbin
	cd debian && fakeroot dpkg-deb --build gndcache .

# rpm building via vagrant
SSHCMD = ssh -o StrictHostKeyChecking=no -i vagrant.key vagrant@127.0.0.1 -p 2222
SCPCMD = scp -o port=2222 -o StrictHostKeyChecking=no -i vagrant.key

rpm: $(TARGETS)
	mkdir -p $(HOME)/rpmbuild/{BUILD,SOURCES,SPECS,RPMS}
	cp ./packaging/gndcache.spec $(HOME)/rpmbuild/SPECS
	cp $(TARGETS) $(HOME)/rpmbuild/BUILD
	./packaging/buildrpm.sh gndcache
	cp $(HOME)/rpmbuild/RPMS/x86_64/gndcache*rpm .

# Helper to build RPM on a RHEL6 VM, to link against glibc 2.12
vagrant.key:
	curl -sL "https://raw.githubusercontent.com/mitchellh/vagrant/master/keys/vagrant" > vagrant.key
	chmod 0600 vagrant.key

# Don't forget to vagrant up :) - and add your public key to the guests authorized_keys
setup: vagrant.key
	$(SSHCMD) "sudo yum install -y sudo yum install http://ftp.riken.jp/Linux/fedora/epel/6/i386/epel-release-6-8.noarch.rpm"
	$(SSHCMD) "sudo yum install -y golang git rpm-build"
	$(SSHCMD) "mkdir -p /home/vagrant/src/github.com/miku"
	$(SSHCMD) "cd /home/vagrant/src/github.com/miku && git clone https://github.com/miku/gndcache.git"

rpm-compatible: vagrant.key
	$(SSHCMD) "cd /home/vagrant/src/github.com/miku/gndcache && GOPATH=/home/vagrant go get ./..."
	$(SSHCMD) "cd /home/vagrant/src/github.com/miku/gndcache && git pull origin master && pwd && GOPATH=/home/vagrant make clean rpm"
	$(SCPCMD) vagrant@127.0.0.1:/home/vagrant/src/github.com/miku/gndcache/*rpm .

# local rpm publishing
REPOPATH = /usr/share/nginx/html/repo/CentOS/6/x86_64

publish: rpm-compatible
	cp gndcache-*.rpm $(REPOPATH)
	createrepo $(REPOPATH)
