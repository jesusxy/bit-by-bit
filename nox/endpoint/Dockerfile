# endpoin/Dockerfile

FROM ubuntu:latest

ENV DEBIAN_FRONTEND=noninteractive

# update packages and install auditd and procps (for debugging)
# what is the difference between linux-headers-$(uname -r) and generic
RUN apt-get update && apt-get install -y bpfcc-tools linux-headers-$(uname -r)

# We pipe the output to a log file in the shared volume.
# is this where we would redirect to a file?
CMD ["/bin/bash", "-c", "/usr/sbin/execsnoop -T >> /var/log/nox/audit.log"]