FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Core tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git jq unzip wget gnupg software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# Amazon Corretto 17 (Java)
RUN curl -fsSL https://apt.corretto.aws/corretto.key | gpg --dearmor -o /usr/share/keyrings/corretto.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/corretto.gpg] https://apt.corretto.aws stable main" > /etc/apt/sources.list.d/corretto.list && \
    apt-get update && apt-get install -y java-17-amazon-corretto-jdk && \
    rm -rf /var/lib/apt/lists/*
ENV JAVA_HOME=/usr/lib/jvm/java-17-amazon-corretto

# Python 3.11
RUN add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && apt-get install -y python3.11 python3.11-venv python3-pip && \
    update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 && \
    pip3 install pipenv && \
    rm -rf /var/lib/apt/lists/*

# Node.js 22
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Gradle 8.12.1
RUN curl -fsSL https://services.gradle.org/distributions/gradle-8.12.1-bin.zip -o /tmp/gradle.zip && \
    unzip /tmp/gradle.zip -d /opt && \
    ln -s /opt/gradle-8.12.1/bin/gradle /usr/local/bin/gradle && \
    rm /tmp/gradle.zip

# lychee (link checker)
RUN curl -fsSL https://github.com/lycheeverse/lychee/releases/latest/download/lychee-x86_64-unknown-linux-gnu.tar.gz | \
    tar xz -C /usr/local/bin

# Codecov uploader
RUN curl -fsSL https://uploader.codecov.io/latest/linux/codecov -o /usr/local/bin/codecov && \
    chmod +x /usr/local/bin/codecov

# Docker CLI (for DinD sidecar usage)
RUN curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-27.5.1.tgz | \
    tar xz --strip-components=1 -C /usr/local/bin docker/docker

WORKDIR /workspace
