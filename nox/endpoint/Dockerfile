# endpoin/Dockerfile

FROM alpine:latest

# update packages and install auditd and procps (for debugging)
RUN apk update && \
    apk add audit

RUN mkdir -p /etc/audit/rules.d/

# add a basic audit rule to log all command executions
RUN echo "-a always,exit -F arch=b64 -S execve -k command_execution" >> /etc/audit/rules.d/audit.rules

CMD rm -f /etc/audit/rules.d/*.rules && /usr/sbin/auditd -n


