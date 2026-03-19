FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git jq unzip gnupg software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# Amazon Corretto 17 (for Java analysis)
RUN curl -fsSL https://apt.corretto.aws/corretto.key | gpg --dearmor -o /usr/share/keyrings/corretto.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/corretto.gpg] https://apt.corretto.aws stable main" > /etc/apt/sources.list.d/corretto.list && \
    apt-get update && apt-get install -y java-17-amazon-corretto-jdk && \
    rm -rf /var/lib/apt/lists/*

# Python 3.11 (for Python analysis)
RUN add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && apt-get install -y python3.11 && \
    rm -rf /var/lib/apt/lists/*

# Node.js 22 (for JS/TS analysis)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# CodeQL CLI
RUN CODEQL_VERSION=$(curl -s https://api.github.com/repos/github/codeql-cli-binaries/releases/latest | jq -r .tag_name) && \
    curl -fsSL "https://github.com/github/codeql-cli-binaries/releases/download/${CODEQL_VERSION}/codeql-linux64.zip" -o /tmp/codeql.zip && \
    unzip /tmp/codeql.zip -d /opt && \
    ln -s /opt/codeql/codeql /usr/local/bin/codeql && \
    rm /tmp/codeql.zip

WORKDIR /workspace
