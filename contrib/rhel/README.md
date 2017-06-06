# How to use these files

In general these files should be used in the same way as the [GCE/AWS quickstart scripts](../../hack/quickstart/) from which they're derived, with one addition: before running the init-{master,worker} scripts against it, each node needs the prep-rhel-node.sh script run on it as root.  This will set it up with the necessary services, users and other configuration to allow the self-hosted install to run successfully.

# Basic workflow

1. Create instances
2. Modify instance/cloud firewalls as needed
3. Run the prep-rhel-node.sh script against all instances
4. Run the init-master.sh script against the designated master node
5. Run the init-worker.sh script against the designated worker node(s)
6. Check Kubernetes environment is running
7. Deploy Kubernetes app(s)
