# endpoin/Dockerfile

FROM ubuntu:latest

ENV DEBIAN_FRONTEND=noninteractive

# update packages and install auditd and procps (for debugging)
RUN apt-get update && apt-get install -y bpfcc-tools linux-headers-$(uname -r)

# We pipe the output to a log file in the shared volume.
CMD ["/usr/sbin/execsnoop", "-T"]