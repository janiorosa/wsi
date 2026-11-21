# WSI — Sistema de Eventos de TCC

Sistema web do **IFMG Campus Ouro Branco** para organizar os eventos de apresentação dos Trabalhos de Conclusão de Curso (TCC) do Bacharelado em Sistemas de Informação.

## O que o sistema faz

- **Eventos**: a administração cadastra o evento (data, horário e local da apresentação).
- **Alunos** cadastram o TCC (título e resumo), enviam o artigo e a apresentação em PDF e escolhem o orientador.
- **Docentes** orientam alunos, montam a banca, associam o trabalho ao evento e avaliam os trabalhos para os quais foram designados, tudo no mesmo perfil.
- **Notas**: cada critério vale de 0 a 100, sem pesos. A nota de cada avaliador é a média dos critérios, e a nota final é a média dos avaliadores.
- **Documentos automáticos**: o sistema gera a **ATA de defesa** e a **Declaração anti-plágio** no modelo institucional, sem ninguém precisar compilar nada.
- **Assinaturas da ATA**: cada ATA segue a ordem aluno → orientador → examinadores. Cada pessoa baixa, assina e envia o PDF quando chega a sua vez.
- **Anais e apresentações**: o docente apresentador junta todos os artigos em um único PDF (anais) e todas as apresentações em outro, confere e publica.
- **Página pública** do evento com programação, anais e apresentações. ATAs, notas e declarações **nunca** são públicas.

## Quem usa

| Perfil | Resumo |
|---|---|
| ALUNO | Envia o TCC, gera e envia a declaração, inicia a assinatura da própria ATA. |
| DOCENTE | Orienta e avalia, sem precisar trocar de modo. Pode ser apresentador de um evento. |
| ADMIN | Cadastra eventos e usuários, aprova docentes, configura critérios e dados institucionais. |

Um docente também pode ser administrador. Nesse caso, ele escolhe, ao entrar, se acessa como ADMIN ou como DOCENTE.

## Onde encontrar mais informações

- [O que é este projeto](project.md)
- [Como executar](running.md)
- [Regras de negócio](business-rules.md) · [Permissões](permissions.md) · [Documentos gerados](documents.md)
- [Arquitetura](architecture.md) · [Banco de dados](database.md) · [API](api.md)
- [Implantação, backup e restauração](deployment.md) · [Decisões](decisions.md)
