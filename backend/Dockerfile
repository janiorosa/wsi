# syntax=docker/dockerfile:1
# Imagem do backend: API Go + trabalhador da fila + XeLaTeX + qpdf/poppler.
# Nenhuma dependência precisa ser instalada no servidor hospedeiro.

ARG VERSAO_GO=1.23.4
ARG VERSAO_DEBIAN=bookworm

# ---------- Pacotes de geração de documentos (reutilizados nos estágios final e de testes)
FROM debian:${VERSAO_DEBIAN}-slim AS pacotes-documentos
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      texlive-xetex texlive-latex-recommended texlive-latex-extra texlive-plain-generic \
      texlive-lang-portuguese texlive-fonts-recommended lmodern fonts-lmodern fontconfig \
      qpdf poppler-utils ca-certificates tzdata curl \
 && rm -rf /var/lib/apt/lists/*
ENV TZ=America/Sao_Paulo
# Verificação na construção: falha aqui se faltar qualquer pacote usado pelos templates.
RUN mkdir /tmp/verificacao && cd /tmp/verificacao \
 && printf '%s\n' '\documentclass[a4paper,12pt]{article}' '\usepackage{fontspec}' '\usepackage[brazil]{babel}' \
      '\usepackage{graphicx}' '\usepackage[normalem]{ulem}' '\usepackage{framed}' '\usepackage{fancyhdr}' \
      '\usepackage[a4paper]{geometry}' '\begin{document}' 'Ação – ATA nº 1 \textbf{\uline{Conclusão “TCC”}}' \
      '\begin{framed}Moldura\end{framed}' '\end{document}' > t.tex \
 && xelatex -no-shell-escape -interaction=nonstopmode -halt-on-error t.tex > /dev/null \
 && ! grep -q 'Missing character' t.log \
 && cd / && rm -rf /tmp/verificacao

# ---------- Compilação do binário Go
FROM golang:${VERSAO_GO}-${VERSAO_DEBIAN} AS construcao
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/servidor ./cmd/servidor

# ---------- Testes (docker compose --profile testes run --rm testes-backend)
FROM golang:${VERSAO_GO}-${VERSAO_DEBIAN} AS testes
ENV DEBIAN_FRONTEND=noninteractive TZ=America/Sao_Paulo
RUN apt-get update && apt-get install -y --no-install-recommends \
      texlive-xetex texlive-latex-recommended texlive-latex-extra texlive-plain-generic \
      texlive-lang-portuguese texlive-fonts-recommended lmodern fonts-lmodern fontconfig \
      qpdf poppler-utils tzdata \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go mod tidy
CMD ["go", "test", "-count=1", "-p", "1", "./..."]

# ---------- Imagem final
FROM pacotes-documentos AS final
RUN useradd --system --uid 10001 --home-dir /app --shell /usr/sbin/nologin wsi \
 && mkdir -p /app /dados && chown wsi:wsi /dados
WORKDIR /app
COPY --from=construcao /out/servidor /app/servidor
COPY templates /app/templates
COPY openapi.yaml /app/openapi.yaml
ENV WSI_DIRETORIO_DADOS=/dados \
    WSI_DIRETORIO_TEMPLATES=/app/templates/documentos \
    WSI_ENDERECO_HTTP=:8080
USER wsi
VOLUME ["/dados"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD curl -fsS http://localhost:8080/api/v1/saude || exit 1
CMD ["/app/servidor"]
