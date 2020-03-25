FROM alpine:3.11

# Run tini as PID 1 and avoid signal handling issues
ADD https://github.com/krallin/tini/releases/download/v0.18.0/tini-static-amd64 /usr/local/bin/tini

# Add exec permissions
RUN chmod +x /usr/local/bin/tini

# Add files for the sidecar
RUN mkdir -p /sidecar

# Jolokia plugin to sidecar for JMX<->HTTP
ADD "http://search.maven.org/remotecontent?filepath=org/jolokia/jolokia-jvm/1.6.0/jolokia-jvm-1.6.0-agent.jar" /sidecar/jolokia.jar
# Add tini to sidecar
RUN cp /usr/local/bin/tini /sidecar/tini

# Add operator binary
COPY manager /usr/local/bin/scylla-operator
RUN chmod +x /usr/local/bin/scylla-operator

# Add executables to sidecar folder
RUN cp /usr/local/bin/scylla-operator /sidecar/scylla-operator

ENTRYPOINT ["tini", "--", "scylla-operator"]