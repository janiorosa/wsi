# WSI — Sistema de Eventos de TCC (IFMG Campus Ouro Branco)

Sistema web para eventos de apresentação de TCC: submissão, orientação, bancas, avaliação, geração automática da ATA e da declaração anti-plágio (XeLaTeX), assinaturas sequenciais e publicação dos anais.

```bash
cp .env.exemplo .env      # ajuste as senhas
docker compose up -d --build
# http://localhost:8080
```

Documentação completa na pasta [`DOC/`](DOC/readme.md). Especificação: `especificacao-sistema-eventos-tcc-v3.md`.
