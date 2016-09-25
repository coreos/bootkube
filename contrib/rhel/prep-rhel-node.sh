#!/bin/bash

# Prerequisites for CoreOS Self-Hosted Kubernetes Install

# Self-hosted Kubernetes install requires several things:

# * systemd, etcd, flannel, docker and rkt
# * a user named "core" with passwordless sudo permissions
# * the CoreOS kubelet-wrapper script placed in /usr/lib/coreos
# * SELinux, if in use, turned off.

# This script will automate these changes.  In addition, various ports 
# need to be opened, which you will need to handle for your network and 
# hosts:

# * SSH to all hosts from management hosts
# * 2379 to all etcd hosts from Kubernetes hosts and any other hosts 
#   running etcd proxies
# * 2380 to all etcd hosts from all other etcd peers
# * 443 to all Kubernetes masters from all Kubernetes and management 
#   hosts
# * 30556 and 32000 to all Kubernetes workers from all management hosts
# * whatever nodePorts you open for your applications to all Kubernetes 
#   workers from anything connecting to them

# This script should be run as root or using sudo.

# If systemd isn't there, die immediately

if ! $(systemd-analyze 2>&1 > /dev/null); then
  echo "Kubernetes self-hosted install is only supported with systemd."
  exit 1
fi

# Set some variables to default values

LNPATH="/usr/bin/ln"

CLEANUP_LIST=""

COREOS_ENV_FILE=${COREOS_ENV_FILE:-/var/coreos/metadata}
COREOS_ENV_DIR=$(dirname $COREOS_ENV_FILE)
BOOTKUBE_ENV_FILE=${BOOTKUBE_ENV_FILE:-${COREOS_ENV_DIR}/metadata-bootkube.conf}

RKT_VER="v1.15.0"
RKT_RPM_VER="1.15.0-1"
RKT_USER="core"
RKT_S1_DIR="/usr/lib/rkt/stage1-images"
RKT_RPM_URL="https://github.com/coreos/rkt/releases/download/${RKT_VER}/rkt-${RKT_RPM_VER}.x86_64.rpm"
RKT_RELEASE_URL="https://github.com/coreos/rkt/releases/download/${RKT_VER}/rkt-${RKT_VER}.tar.gz"

# Are we on RHEL 7, CoreOS or something unsupported?

if [ -f /etc/os-release ]; then
  source <(cat /etc/os-release)
  OS_NAME=$ID
  OS_VER_MAJOR=${VERSION_ID%.*}
  # Coreos has two dots in its version string, so we strip from the last dot again
  OS_VER_MAJOR=${OS_VER_MAJOR%.*}
else
  echo "Your distribution does not have an /etc/os-release file, which means it's unsupported for Kubernetes self-hosted install."
  exit 1
fi

case $OS_NAME in
  rhel)
    if [ "$OS_VER_MAJOR" -lt "7" ]; then 
      echo "Your version of RHEL is not supported for Kubernetes self-hosted"
      echo "install."
      exit 1
    else
      PKG_INSTALL="yum install -y -q"
      PKG_LIST="docker etcd flannel https://dl.fedoraproject.org/pub/epel/7/x86_64/j/jq-1.5-1.el7.x86_64.rpm https://dl.fedoraproject.org/pub/epel/7/x86_64/o/oniguruma-5.9.5-3.el7.x86_64.rpm"
      rpm --import http://download.fedoraproject.org/pub/epel/RPM-GPG-KEY-EPEL-7
      # Persistently set SELinux to non-enforcing
      setenforce 0
      if [ -f /etc/sysconfig/selinux ] && $(grep -q "SELINUX=enforcing" /etc/sysconfig/selinux); then
        sed -i.bak -e 's/SELINUX=enforcing/SELINUX=permissive/' /etc/sysconfig/selinux
      fi
    fi
    ;;
  coreos)
    if [ "$OS_VER_MAJOR" -lt "962" ]; then
      echo "Your version of CoreOS is not supported for Kubernetes" 
      echo "self-hosted install."
      exit 1
    else
      echo "You don't need to run this script against a version of CoreOS"
      echo "that's supported for self-install."
      exit 0
    fi
    ;;
  *)
    echo "Your distribution is not supported for Kubernetes self-hosted install."
    exit 1
    ;;
esac

# Pull the metadata from GCE or AWS

if [ $(curl -ss --connect-timeout 1 -f -H "Metadata-Flavor: Google" "http://169.254.169.254/computeMetadata/v1/instance/id") ]; then
  COREOS_OEM="gce"
elif [ $(curl -ss --connect-timeout 1 -f http://169.254.169.254/latest/meta-data/ami-id) ]; then
  COREOS_OEM="ec2"
else
  if [ -f $BOOTKUBE_ENV_FILE ]; then
    COREOS_OEM="bare"
    source $BOOTKUBE_ENV_FILE
  else
    echo "Your cloud provider, if any, is not (yet) supported by this script."
    echo "If you would like to use this script anyway, create the file"
    echo "$BOOTKUBE_ENV_FILE with variable declarations in it for"
    echo "COREOS_PUBLIC_IPV4 and COREOS_PRIVATE_IPV4."
    exit 1
  fi
fi

case $COREOS_OEM in
  gce)
    COREOS_PUBLIC_IPV4=$(curl -ss -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip)
    COREOS_PRIVATE_IPV4=$(curl -ss -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip)
    ;;
  ec2)    
    COREOS_PUBLIC_IPV4=$(curl -ss http://169.254.169.254/latest/meta-data/public-ipv4)
    COREOS_PRIVATE_IPV4=$(curl -ss http://169.254.169.254/latest/meta-data/local-ipv4)
    ;;
esac

# Write out metadata for the self-install script to use

mkdir -p $COREOS_ENV_DIR
mkdir -p $(dirname $BOOTKUBE_ENV_FILE)
echo "COREOS_PUBLIC_IPV4=${COREOS_PUBLIC_IPV4}" > $BOOTKUBE_ENV_FILE
echo "COREOS_PRIVATE_IPV4=${COREOS_PRIVATE_IPV4}" >> $BOOTKUBE_ENV_FILE
echo "LNPATH=${LNPATH}" >> $BOOTKUBE_ENV_FILE
cat $COREOS_ENV_DIR/metadata-*.conf > $COREOS_ENV_FILE

# Install etcd, docker, flannel and jq

$PKG_INSTALL $PKG_LIST

# Red Hat changes the default flannel network config prefix, so if we 
# detect that, we change it back to keep cross-distro compatibility

if [ -f /etc/sysconfig/flanneld ]; then
  sed -i.bak -e 's/FLANNEL_ETCD_KEY=\"\/atomic.io\/network\"/FLANNEL_ETCD_KEY=\"\/coreos.com\/network\"/' /etc/sysconfig/flanneld
  echo "Edited flannel network config to use /coreos.com/network."
fi

# Some basic user setup:

# If the core user doesn't exist, create it.

if ! $(id core > /dev/null); then
  useradd -m core
  echo "Created 'core' user."
else
  echo "'core' user already exists."
fi

# If the user has no .ssh directory, create it

if ! [ -d ~core/.ssh ]; then
  sudo -u core mkdir -p ~core/.ssh
  sudo -u core chmod go-rwx ~core/.ssh
fi

# If the core user has no SSH authorized_keys, copy ours if it exists

if [ ! -f ~core/.ssh/authorized_keys ]; then
  if [ -f ~/.ssh/authorized_keys ]; then
    cat ~/.ssh/authorized_keys | sudo -u core sh -c "cat - >> ~core/.ssh/authorized_keys"
    sudo -u core chmod go-rwx ~core/.ssh/authorized_keys
    echo "Added authorized_keys from $USER to 'core' user."
  fi
fi

# If the core user doesn't have passwordless sudo, add that.

if ! $(sudo -l -U core | grep -q "NOPASSWD: ALL"); then
  if [ -d /etc/sudoers.d ]; then 
    if [ -f /etc/sudoers.d/core]; then
      echo "sudoers file snippet fore 'core' user already exists; not"
      echo "changing."
    else
      echo "core ALL=NOPASSWD: ALL" >> /etc/sudoers.d/core
      echo "Configured passwordless sudo for core user."
    fi
  else
    echo "No sudoers config directory detected.  You will need to configure" 
    echo "passwordless sudo for the 'core' user manually if you have not"
    echo "already done so."
  fi
else
  echo "'core' user already has passwordless sudo."
fi

# Install rkt
if rkt version > /dev/null 2>&1; then
  echo "rkt already installed."
else
  case $OS_NAME in
    rhel)
      if yum install -y $RKT_RPM_URL ; then 
        usermod -a -G rkt,rkt-admin $RKT_USER
        echo "Installed rkt."
      else
        echo "rkt install failed."
        exit 1
      fi
      ;;
    *)
      groupadd rkt
      groupadd rkt-admin
      usermod -a -G rkt,rkt-admin $RKT_USER
      curl -ss -L $RKT_RELEASE_URL > rkt-${RKT_VER}.tar.gz
      tar zxf rkt-${RKT_VER}.tar.gz
      cp rkt-${RKT_VER}/rkt /usr/local/bin/rkt
      ln -sf /usr/local/bin/rkt /usr/bin/rkt

      # Some distributions use totally separate /usr/lib and /usr/lib64, some 
      # link one to the other, and some just use /usr/lib.

      if [ -d /usr/lib64 ]; then
        mkdir -p /usr/lib64/rkt/stage1-images
      else
        mkdir -p /usr/lib/rkt/stage1-images
      fi
      if [ ! -d /usr/lib/rkt ] && [ -d /usr/lib64/rkt ]; then
        ln -s /usr/lib64/rkt /usr/lib/rkt
      fi

      cp rkt-${RKT_VER}/*.aci ${RKT_S1_DIR}
      cp rkt-${RKT_VER}/init/systemd/tmpfiles.d/* /etc/tmpfiles.d
      systemd-tmpfiles --create

      (rkt version && echo "Installed rkt.") || echo "rkt install failed." && exit 1

      CLEANUP_LIST="$CLEANUP_LIST rkt-${RKT_VER}.tar.gz rkt-${RKT_VER}"
      ;;
  esac
fi

# TODO: should we pre-trust keys for things?

# Install /usr/lib/coreos/kubelet-wrapper

mkdir -p /usr/lib/coreos

curl -ss https://raw.githubusercontent.com/coreos/coreos-overlay/master/app-admin/kubelet-wrapper/files/kubelet-wrapper > kubelet-wrapper

sed -i -e 's/source=\/usr\/share\/ca-certificates/source=\/etc\/ssl\/certs/;s/source=\/usr\/lib\/os-release/source=\/etc\/os-release/' kubelet-wrapper
mv kubelet-wrapper /usr/lib/coreos
chmod +x /usr/lib/coreos/kubelet-wrapper
CLEANUP_LIST="$CLEANUP_LIST kubelet-wrapper"
echo "Installed kubelet-wrapper."

# Clean up
rm -rf $CLEANUP_LIST && echo "Cleaned up: "$CLEANUP_LIST

echo "Prep complete."
