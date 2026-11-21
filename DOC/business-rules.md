# Regras de negócio implementadas

## Cadastro e acesso
- Aluno se cadastra livremente e a conta fica ativa na hora. Não há aprovação nem confirmação de e-mail; o e-mail só é validado quanto ao formato.
- Docente se cadastra livremente, mas o perfil DOCENTE só existe após a **aprovação do ADMIN**. Antes disso, o docente vê apenas “aguardando aprovação”.
- ADMIN cadastra alunos, docentes (já aprovados, com opção de também serem ADMIN) e administradores. A matrícula/SIAPE é obrigatória. A pessoa cria a própria senha no **Primeiro acesso**.
- O docente pode cadastrar um aluno de forma simplificada (nome, e-mail e matrícula obrigatórios). O aluno é vinculado como orientando e cria a senha no Primeiro acesso.
- **Primeiro acesso**: a pessoa informa nome completo, e-mail e matrícula/SIAPE (comparação sem diferenciar maiúsculas, acentos, espaços extras, pontuação e zeros à esquerda) e cria a senha. Só vale para contas **sem senha**. Cinco tentativas erradas bloqueiam por 15 minutos.
- **Esqueci a senha**: somente o ADMIN pode **redefinir o acesso**. A senha é apagada, as sessões são encerradas e a pessoa refaz o primeiro acesso. Um ADMIN não redefine a própria conta. O sistema não usa códigos nem e-mails.

## Orientação
- Um orientador principal ativo por aluno. Vínculos encerrados permanecem no histórico.
- Aluno sem orientador escolhe um docente. Se já tiver orientador, a troca exige **confirmação explícita** e encerra o vínculo anterior.
- Docente vincula somente alunos **sem** orientador. ADMIN vincula qualquer aluno (a troca também exige confirmação).
- Ao mudar o orientador, os trabalhos em andamento **sem ATA** passam a ter o novo orientador como integrante da banca.

## Eventos e trabalhos
- O orientador só associa o trabalho a evento (ou sessão) cuja apresentação **ainda não aconteceu** e que esteja recebendo trabalhos (não rascunho, encerrado ou publicado). O ADMIN pode abrir uma exceção, com justificativa registrada.
- Cada trabalho tem **um único evento atual**, o que torna impossível a ambiguidade entre eventos. Se o evento tiver sessões, a sessão é obrigatória.
- Um trabalho em andamento por aluno. Título e resumo são obrigatórios para a submissão completa; o link do Overleaf é opcional.
- Artigo e apresentação são enviados de forma independente, e **todas as versões ficam guardadas**.
- Depois que a ATA é gerada, título, resumo, evento e banca ficam bloqueados. Para alterar, o orientador reinicia o processo da ATA (com registro em auditoria).

### Estados do trabalho (derivados automaticamente pelo backend)
`rascunho` → `submetido` (sem banca e sem evento) / `aguardando_avaliadores` (com evento, sem examinadores) → `aguardando_avaliacoes` → `avaliacoes_concluidas` → `ata_aguardando_geracao` → `ata_gerada` (aguardando o aluno) → `assinaturas_em_andamento` → `documentacao_pendente` (ATA finalizada, falta a declaração assinada) → `concluido`. Cada mudança fica em `historico_estados_trabalho`.

## Banca e avaliação
- O orientador integra a banca automaticamente e designa os examinadores, em ordem e em **quantidade variável** (pelo menos 1). O mesmo docente não pode aparecer duas vezes.
- Cada avaliador vê e avalia somente os trabalhos para os quais foi designado (inclusive os dos próprios orientandos).
- Os critérios gerais são copiados para o trabalho (fotografia histórica). O orientador pode criar **critérios extras** só para aquele trabalho. Após a primeira avaliação finalizada, os critérios ficam **congelados**.
- **Cálculo (regra `media-simples-v1`)**:
  - cada critério vale de 0 a 100, com até 2 casas decimais (valores fora da faixa são rejeitados);
  - nota do avaliador = média aritmética simples dos critérios (critério único = a própria nota);
  - nota final = média aritmética simples das notas dos avaliadores obrigatórios, incluindo o orientador;
  - não há pesos nem soma; a aritmética é exata em centésimos, com arredondamento **meio para cima** a 2 casas em cada média;
  - exemplo: avaliadores 90,00, 70,00 e 90,00 → 83,33;
  - **aprovação: nota final ≥ 60,00**. A ATA registra “deliberou pela **Aprovação**” (≥ 60,00) ou “deliberou pela **Reprovação**” (< 60,00).
- O critério geral **“Nota”** (0 a 100) existe sempre: é criado automaticamente, não pode ser excluído, renomeado nem desativado, e é aplicado a todos os trabalhos. Assim, todos os docentes da banca atribuem a sua Nota. O ADMIN pode cadastrar outros critérios gerais (que entram na média simples).
- A avaliação finalizada só pode ser **reaberta pelo orientador**, com motivo registrado, e somente antes da ATA.
- Uma avaliação pendente impede a ATA. Quando a última avaliação obrigatória termina e o trabalho tem evento, **a ATA é gerada automaticamente**.

## ATA e assinaturas
- A nota final e a ordem dos signatários são **congeladas** quando a ATA é criada: 1 aluno, 2 orientador (“Professor Orientador”), 3… examinadores (“Examinador 1”, “Examinador 2”…).
- Data, horário e local impressos vêm **sempre** do evento ou da sessão atual do trabalho, nunca do relógio do servidor nem da data de assinatura.
- O fluxo é **independente por trabalho**. Um docente participa de vários fluxos ao mesmo tempo, sem interferência entre eles.
- Somente o signatário da vez baixa e envia. O envio exige a posição atual, para que um duplo clique ou um reenvio não avance duas posições. Um arquivo rejeitado mantém o mesmo signatário.
- Cada envio válido **substitui** o arquivo assinado anterior, que é apagado. Ficam apenas os metadados: quem enviou, ordem, data/hora e SHA-256.
- O PDF original pode ser regenerado enquanto ninguém assinou. Depois disso, só há o **reinício auditado**: a ATA é cancelada e outra é gerada.
- O sistema não afirma que a assinatura enviada equivale à assinatura digital ICP-Brasil.

## Declaração anti-plágio
- Gerada pelo aluno com o texto jurídico preservado, a data de **emissão** por extenso e a cidade configurada.
- O aluno envia o PDF assinado (várias versões são permitidas). Somente o aluno e o orientador ativo acessam a declaração.

## Anais e apresentações
- O APRESENTADOR gera o PDF das apresentações e os anais (artigos), na **ordem da programação**, usando sempre a versão mais recente de cada arquivo.
- Arquivos ausentes ou inválidos ficam de fora e aparecem no relatório de inclusão. ATAs, notas, avaliações e declarações nunca entram.
- O documento fica “aguardando revisão”. O apresentador confere e **publica**, sem aprovação do ADMIN. Uma nova publicação torna a anterior “substituída”.
