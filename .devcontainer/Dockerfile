FROM mcr.microsoft.com/devcontainers/go:2-1.24-bookworm

ARG REMOTE_USER
ARG REMOTE_UID
ARG REMOTE_GID

# get is more appropriate for scripted use
RUN apt-get update && apt-get -y upgrade && apt-get -y install adduser

RUN addgroup --gid ${REMOTE_GID} ${REMOTE_USER} && \
    adduser --disabled-password --uid ${REMOTE_UID} --gid ${REMOTE_GID} ${REMOTE_USER}  && \
    getent group netdev || groupadd netdev && \
    adduser ${REMOTE_USER} sudo  && \
    adduser ${REMOTE_USER} netdev && \
    echo "${REMOTE_USER} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers 


ENV HOME=/home/${REMOTE_USER}
USER ${REMOTE_USER}


