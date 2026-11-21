# ESPECIFICAÇÃO FUNCIONAL E TÉCNICA
## Sistema WEB de Gestão de Eventos Acadêmicos, TCCs, Avaliações e Documentos

**Instituição inicial:** IFMG — Campus Ouro Branco  
**Nome do projeto:** definir posteriormente  
**Versão:** 2.0 — consolidada com as decisões do responsável pelo sistema  
**Idioma:** Português do Brasil  
**Fuso horário:** `America/Sao_Paulo`  
**Público:** administradores, docentes, alunos e apresentadores de eventos

---

# 1. Instrução principal ao agente de desenvolvimento

Atue como arquiteto de software e engenheiro de software sênior. Desenvolva um sistema WEB completo, funcional, seguro, testável e documentado para gerenciar eventos acadêmicos de apresentação de Trabalhos de Conclusão de Curso (TCC).

O sistema deverá cobrir o ciclo completo: cadastro e autenticação de usuários, orientação de alunos, criação de eventos, submissão de trabalhos, designação de avaliadores, avaliação, cálculo de notas, geração automática de documentos institucionais, fluxo sequencial de assinaturas, consolidação de artigos e apresentações e publicação dos anais.

**Não entregue somente protótipos, telas estáticas ou CRUDs demonstrativos.** Todos os botões, formulários, filtros, permissões, operações de banco, uploads, downloads, geração de PDFs e fluxos devem funcionar de ponta a ponta.

Antes de implementar:
1. Leia integralmente esta especificação.
2. Inspecione os arquivos de referência fornecidos, especialmente os dois arquivos LaTeX da ATA e da declaração antiplágio, os PDFs correspondentes e a imagem do brasão.
3. Inspecione a estrutura do repositório, caso já exista código.
4. Apresente um plano de implementação por fases e identifique apenas as dúvidas que realmente bloqueiem decisões de negócio.
5. Não descarte funcionalidades existentes sem justificativa.
6. Implemente em etapas, mantendo o sistema executável ao final de cada fase.
7. Crie testes automatizados para regras de negócio, permissões, cálculos, assinaturas e geração de documentos.
8. Não invente dados pessoais, SIAPEs, matrículas, notas, textos institucionais ou documentos.
9. Não altere regras expressamente definidas nesta especificação por conveniência técnica.

O objetivo é um sistema simples de usar, rápido, visualmente agradável e centrado nos eventos e na geração dos documentos. A complexidade deve ficar na implementação, não nas telas utilizadas pelos servidores e alunos.

# 2. Visão do produto

O sistema é uma plataforma WEB para organizar eventos acadêmicos de apresentação e defesa de TCCs. Um evento reúne trabalhos, alunos, docentes, bancas avaliadoras e documentos institucionais.

O fluxo geral é:

1. Usuários se cadastram ou são cadastrados.
2. Docentes recebem aprovação administrativa para atuar como DOCENTE.
3. Alunos e docentes estabelecem vínculos de orientação.
4. ADMIN cria um evento.
5. Aluno submete título, resumo, artigo e apresentação, em momentos independentes.
6. Orientador associa o trabalho a um evento futuro.
7. Orientador designa os docentes avaliadores.
8. Cada docente avaliador avalia o trabalho.
9. O sistema calcula a nota final.
10. O sistema gera automaticamente a ATA em PDF.
11. A ATA percorre o fluxo sequencial de assinaturas.
12. O aluno envia a declaração antiplágio assinada.
13. O APRESENTADOR gera o PDF consolidado das apresentações e os anais dos artigos.
14. Os documentos consolidados são revisados e publicados.
15. A página pública do evento disponibiliza os anais e as apresentações, nunca as atas individuais.

## 2.1 Princípios fundamentais

- O evento é o centro da navegação e da organização.
- A geração automática de documentos é uma função central, não acessória.
- A interface deve ser rápida, clara e exigir poucos cliques.
- Ações importantes devem aparecer em botões destacados na própria tela e também no menu. Essa redundância é intencional.
- O sistema deve orientar o usuário sobre a próxima ação necessária.
- Não exigir conhecimento de LaTeX, Docker, PDF ou tecnologia dos usuários.
- A compilação LaTeX ocorrerá automaticamente dentro do ambiente Docker.
- Nenhum usuário deverá executar comandos, compilar arquivos `.tex` ou gerar PDFs manualmente.
- O sistema terá poucos usuários e poucos trabalhos por evento (aproximadamente 15 a 20 alunos). Não superdimensionar a arquitetura.
- A fila de documentos será assíncrona e persistida no PostgreSQL.
- Não utilizar Redis, Kafka, RabbitMQ ou outros serviços de mensageria.
- Não utilizar microsserviços nem Kubernetes.

# 3. Stack tecnológica e arquitetura

## 3.1 Tecnologias

| Camada | Tecnologia |
|---|---|
| Frontend | React + TypeScript |
| Ferramenta de build | Vite |
| Componentes visuais | Material UI (MUI) |
| Formulários | React Hook Form + Zod |
| Backend | Go (Golang) |
| API | REST + JSON |
| Banco de dados | PostgreSQL |
| Migrations | Ferramenta de migrations versionadas |
| Acesso a dados | SQL explícito, sqlc ou solução equivalente |
| Documentos institucionais | LaTeX + XeLaTeX |
| Manipulação de PDFs | Biblioteca confiável compatível com Go, como pdfcpu, ou ferramenta equivalente |
| Infraestrutura | Docker + Docker Compose |
| Testes backend | Go test |
| Testes frontend | Vitest + Testing Library |
| Testes ponta a ponta | Playwright |
| Documentação da API | OpenAPI/Swagger |

Utilizar versões estáveis e compatíveis, fixadas no projeto. Documentar as versões e o procedimento de atualização.

## 3.2 Arquitetura: monólito modular

Utilizar um **monólito modular**. Não criar microsserviços.

O backend Go será dividido em módulos/domínios: autenticação, usuários, docentes, alunos, orientação, eventos, trabalhos, avaliações, documentos, assinaturas e publicações.

A geração de documentos poderá ser executada por um worker interno, no mesmo projeto/backend, compartilhando o PostgreSQL e o armazenamento persistente.

Não criar infraestrutura complexa para o volume esperado. O sistema deverá suportar com folga a geração de poucos documentos simultâneos e eventos com cerca de 15 a 20 alunos.

## 3.3 Containers Docker

Utilizar Docker Compose com os serviços principais:

1. `frontend`: aplicação React compilada e servida por servidor HTTP apropriado.
2. `backend`: API Go e worker assíncrono.
3. `postgres`: banco PostgreSQL.

O ambiente de execução do backend ou um container interno de processamento de documentos deverá conter:
- XeLaTeX;
- pacotes LaTeX necessários aos templates;
- fontes utilizadas;
- ferramentas/bibliotecas de manipulação de PDF.

A instalação do XeLaTeX e de suas dependências deverá fazer parte da imagem Docker. **Não depender de instalação manual no servidor hospedeiro.**

O processo de geração será disparado pela aplicação e executado automaticamente dentro do Docker. Nenhum operador precisará abrir terminal ou executar `pdflatex`, `xelatex` ou outro comando manualmente.

## 3.4 Persistência

- PostgreSQL em volume nomeado ou bind mount persistente.
- Arquivos enviados e documentos gerados em volume persistente próprio.
- `docker compose down` não pode apagar os dados.
- `docker compose build` e `docker compose up --build` não podem apagar os dados.
- Não armazenar arquivos importantes apenas na camada efêmera de um container.
- Documentar backup e restauração.
- **Atenção:** `docker compose down -v` remove volumes nomeados. Não é possível garantir persistência diante da remoção explícita do volume. Documentar que esse comando não deve ser executado em produção.

## 3.5 Fila assíncrona de documentos

A fila será implementada diretamente no PostgreSQL, sem Redis, Kafka, RabbitMQ ou serviço externo.

Tarefas típicas:
- gerar declaração antiplágio;
- gerar ATA;
- regenerar documento original;
- juntar apresentações;
- gerar anais;
- validar e finalizar documentos.

A fila deverá:
- registrar a tarefa e seus parâmetros;
- ter estados `pending`, `processing`, `completed` e `failed`;
- evitar duplicação causada por duplo clique;
- registrar início, conclusão e erro;
- permitir reprocessamento de falhas;
- permitir consulta de estado pelo frontend;
- sobreviver a reinicializações do backend/container;
- usar transações e bloqueio adequado de registros para impedir que dois workers processem a mesma tarefa.

**Dimensionamento:** utilizar um worker interno com concorrência baixa, inicialmente uma tarefa por vez, configurável para até duas tarefas simultâneas se necessário. O usuário informou que normalmente haverá no máximo dois documentos sendo gerados ao mesmo tempo. Não implementar mecanismos complexos de escalabilidade que não tragam benefício real.

O frontend deve informar que o documento está sendo preparado e atualizar o estado até liberar o download. Não deixar o usuário esperando uma requisição HTTP longa.

# 4. Identidade, perfis e permissões

Esta seção é especialmente importante. Implementar exatamente a distinção entre ADMIN, DOCENTE e ALUNO descrita abaixo.

## 4.1 Perfis principais

Existem três perfis principais:

- `ADMIN`
- `DOCENTE`
- `ALUNO`

O perfil `APRESENTADOR` não precisa ser uma identidade independente: é uma atribuição/capacidade de um DOCENTE para um evento específico.

## 4.2 ALUNO: perfil exclusivo

ALUNO é o único perfil que não pode se sobrepor a nenhum outro.

- Um ALUNO não pode ser ADMIN.
- Um ALUNO não pode ser DOCENTE.
- Um ALUNO não pode ser APRESENTADOR.
- Um ALUNO só acessa funcionalidades de aluno.
- Não permitir atribuição de outros perfis a uma conta ALUNO.

## 4.3 DOCENTE: professor e orientador em uma única identidade

Professor e orientador serão reunidos em um único perfil: `DOCENTE`.

**Não criar perfis separados de PROFESSOR e ORIENTADOR. Não exigir que o usuário escolha entre ser professor ou orientador.**

Todo DOCENTE possui automaticamente as duas capacidades, de maneira transparente:

### Capacidades de avaliação (antiga função de professor)
- Ver trabalhos que foram destinados a ele como avaliador.
- Avaliar somente trabalhos para os quais foi designado.
- Consultar avaliações que realizou.
- Participar do fluxo de assinatura da ATA quando for signatário.
- Baixar e enviar a ATA somente quando chegar sua vez de assinar.
- Consultar documentos dos trabalhos nos quais participa, conforme as permissões.

### Capacidades de orientação (antiga função de orientador)
- Ver seus próprios orientandos.
- Ver trabalhos submetidos por seus orientandos.
- Ver título, resumo e arquivos dos trabalhos dos seus orientandos.
- Vincular alunos sem orientador.
- Remover o vínculo de orientação.
- Designar trabalhos de seus orientandos para eventos futuros.
- Designar docentes avaliadores para esses trabalhos.
- Criar critérios extras específicos para um trabalho de seu orientando.
- Consultar avaliações e notas dos trabalhos sob sua orientação.
- Gerar a ATA depois que as avaliações obrigatórias forem concluídas.
- Acompanhar o fluxo de assinaturas.
- Baixar a declaração antiplágio assinada pelo orientando.

### Comportamento transparente

O DOCENTE não deve precisar selecionar um modo `PROFESSOR` ou `ORIENTADOR`.

A interface deverá apresentar automaticamente:
- área `Minhas avaliações` para os trabalhos destinados ao docente;
- área `Meus orientandos` para os alunos sob sua orientação;
- ações contextuais de orientação nas fichas dos próprios orientandos;
- ações de avaliação somente nos trabalhos para os quais foi designado.

Se o docente ainda não possuir orientandos, a área de orientação poderá mostrar uma mensagem simples e a opção de vincular um aluno, sem prejudicar as funções de avaliação.

Se um docente for orientador de um aluno e também avaliador de trabalhos de outros orientadores, ele verá:
- os trabalhos dos próprios orientandos;
- os trabalhos de outros orientadores que foram especificamente destinados a ele para avaliação.

Não deverá ver trabalhos de terceiros que não estejam sob sua orientação nem tenham sido destinados a ele.

**Não criar uma troca manual de perfil entre PROFESSOR e ORIENTADOR. Essas capacidades coexistem na mesma sessão DOCENTE.**

## 4.4 ADMIN: perfil que pode se sobrepor ao DOCENTE

ADMIN é um perfil administrativo independente.

Um usuário pode ser:
- somente ADMIN;
- somente DOCENTE;
- ADMIN + DOCENTE.

ADMIN e DOCENTE podem se sobrepor. Um ou mais docentes poderão também ser administradores.

O usuário ADMIN que não tiver DOCENTE não terá as funções de avaliação e orientação.

O usuário que tiver ADMIN + DOCENTE deverá escolher o contexto de acesso ao entrar no sistema.

### Escolha de contexto após login

Quando um usuário possuir simultaneamente ADMIN e DOCENTE:
1. Informa e-mail e senha normalmente.
2. Após autenticação bem-sucedida, o sistema apresenta uma tela ou popup simples com dois botões:
   - `ENTRAR COMO ADMIN`
   - `ENTRAR COMO DOCENTE`
3. O usuário escolhe o contexto daquela sessão.
4. O sistema apresenta a interface correspondente.

Quando o usuário possuir somente ADMIN, entra diretamente na área administrativa. Quando possuir somente DOCENTE, entra diretamente na área docente.

A escolha de contexto não deve alterar os perfis armazenados no banco. É apenas o contexto de navegação e autorização daquela sessão.

Permitir trocar de contexto de maneira explícita e segura, se o usuário tiver ambos os perfis. Ao trocar, atualizar as permissões efetivas da sessão e a interface.

### Área ADMIN

ADMIN poderá:
- cadastrar e administrar eventos;
- cadastrar usuários;
- aprovar ou rejeitar solicitações de perfil DOCENTE;
- atribuir ou remover o perfil ADMIN;
- atribuir ou remover o perfil DOCENTE, respeitando as regras;
- consultar e administrar alunos;
- consultar e administrar docentes;
- atribuir docentes a eventos como apresentadores;
- cadastrar critérios gerais;
- configurar dados institucionais;
- acompanhar trabalhos, eventos e processamento de documentos;
- consultar logs administrativos.

Por padrão, a área ADMIN não deve exibir as funções docentes. Para um usuário ADMIN + DOCENTE, essas funções ficam disponíveis somente quando ele escolher entrar como DOCENTE.

**Privacidade das ATAs:** mesmo na área ADMIN, não conceder automaticamente acesso ao conteúdo das ATAs individuais. A ATA final é restrita ao aluno avaliado e aos docentes participantes daquela banca. A eventual necessidade de acesso excepcional deve ser definida por regra institucional específica e registrada em auditoria; não presumir acesso irrestrito do ADMIN.

## 4.5 APRESENTADOR

APRESENTADOR é uma atribuição por evento, concedida a um DOCENTE.

O docente designado como apresentador poderá:
- acessar o painel do evento correspondente;
- gerar PDF consolidado das apresentações;
- gerar os anais dos artigos;
- conferir arquivos e pendências;
- publicar os documentos consolidados, se essa permissão lhe for concedida.

O apresentador não recebe acesso às notas ou às ATAs individuais apenas por ser apresentador.

Um docente pode ser apresentador de um evento e avaliador/orientador em outros contextos.

## 4.6 Matriz de acesso e autorização

Todas as permissões devem ser verificadas no backend. Ocultar botões no frontend não é suficiente.

Validar:
- identidade autenticada;
- perfil e contexto da sessão;
- vínculo do usuário com o aluno, trabalho ou evento;
- designação como avaliador;
- designação como apresentador;
- estado do processo;
- permissão para acessar o arquivo específico.

Não permitir acesso por simples conhecimento de URL ou UUID (prevenção de IDOR).

# 5. Cadastro, autenticação e senhas

## 5.1 Campos básicos

Campos cadastrais principais:
- nome completo;
- e-mail;
- telefone;
- identificador institucional.

O identificador institucional poderá ser:
- SIAPE para docente;
- matrícula acadêmica para aluno.

Os demais dados poderão ser opcionais conforme a necessidade institucional.

O e-mail deve ser validado quanto ao formato no frontend e também no backend. **Não será exigida confirmação de endereço por e-mail. Não enviar link de ativação e não implementar verificação de posse do e-mail nesta versão.**

## 5.2 Cadastro de aluno

- Aluno pode se cadastrar livremente pelo site.
- Após preencher os dados obrigatórios e criar senha, a conta fica ativa imediatamente.
- Não exige aprovação administrativa.
- Não exige ativação por e-mail.

## 5.3 Cadastro de docente

- Docente pode solicitar cadastro livremente pelo site.
- O cadastro de docente fica pendente de aprovação.
- Somente ADMIN pode aprovar ou rejeitar o perfil DOCENTE.
- Antes da aprovação, o usuário não pode acessar as funcionalidades docentes.
- O usuário pode possuir uma conta e senha, mas a capacidade DOCENTE só é liberada após aprovação.
- Nenhum outro perfil exige aprovação.

## 5.4 Cadastro pelo ADMIN

ADMIN pode cadastrar diretamente alunos e docentes. Para docente cadastrado diretamente pelo ADMIN, permitir que o administrador conceda/aprove o perfil DOCENTE no próprio fluxo administrativo.

ADMIN pode cadastrar outro administrador ou atribuir o perfil ADMIN a usuário existente.

## 5.5 Cadastro simplificado de aluno pelo docente

O DOCENTE poderá cadastrar um aluno com os dados básicos disponíveis e vinculá-lo como orientando.

O aluno poderá posteriormente completar os dados cadastrais e criar sua senha por um procedimento de primeiro acesso. Esse procedimento deve usar um identificador temporário seguro gerado pelo sistema, sem depender de confirmação de e-mail.

## 5.6 Primeiro acesso e senha

O usuário cria uma senha no cadastro. A partir daí, entra com e-mail e senha.

O ADMIN inicial será criado durante a instalação/bootstrap do sistema. No primeiro acesso, deverá definir sua senha permanente.

Para isso:
- criar uma credencial inicial temporária por configuração segura de instalação ou segredo de uso único;
- não deixar senha padrão fixa no código;
- exigir a definição de senha permanente no primeiro acesso;
- invalidar a credencial inicial depois da definição da senha.

## 5.7 Armazenamento de senha

A senha não deve ser armazenada em texto puro nem criptografada de forma reversível.

**Armazenar hash seguro da senha**, preferencialmente Argon2id, com parâmetros adequados. Essa é a abordagem correta para senhas: o sistema não precisa recuperar a senha original, apenas verificar a senha informada.

Implementar recuperação de senha por procedimento seguro compatível com a ausência de confirmação de e-mail, por exemplo fluxo administrativo de redefinição ou token temporário disponibilizado por canal institucional. Não inventar envio de e-mail.

# 6. Eventos

## 6.1 Cadastro

ADMIN poderá criar eventos com:
- nome;
- edição/ano;
- descrição;
- data específica da apresentação;
- horário específico da apresentação;
- local;
- campus;
- estado;
- prazos de submissão;
- prazo de envio da apresentação;
- apresentador(es);
- informações públicas;
- imagem/capa opcional.

Local inicial sugerido: `Salão de Convenções do Campus IFMG Ouro Branco`.

O local deve ser configurável por evento.

## 6.2 Estados

Estados sugeridos:
- rascunho;
- aberto para submissões;
- em preparação;
- em andamento;
- encerrado;
- publicado.

## 6.3 Regras

- O orientador só poderá associar um trabalho a evento cuja data ainda seja futura.
- Usar fuso `America/Sao_Paulo` para comparações de datas e horários.
- Impedir associação a evento já ocorrido, salvo exceção administrativa explicitamente registrada.
- Permitir ordenar os trabalhos na programação.
- A data, o horário e o local oficiais da apresentação usados na ATA são os cadastrados no evento atual associado ao trabalho. Não criar sobrescrita independente por trabalho para esses campos documentais.
- Se o evento tiver várias sessões ou datas, cadastrar no evento a sessão/data/horário aplicável e associar o trabalho à sessão correspondente. A ATA deverá buscar esses dados da sessão/evento associado, nunca do relógio do servidor.
- Permitir consultar eventos anteriores e suas publicações.

## 6.4 Página pública

Cada evento publicado terá página pública com URL compartilhável.

Pode mostrar:
- nome, descrição, data e local;
- programação;
- lista pública de trabalhos;
- anais;
- PDF consolidado das apresentações.

Nunca publicar:
- ATA individual;
- notas;
- avaliações;
- declaração antiplágio;
- arquivos privados;
- informações pessoais não destinadas à divulgação.

# 7. Orientação de alunos

## 7.1 Vínculo

O vínculo entre aluno e docente poderá ser criado:
- pelo próprio aluno;
- pelo próprio docente;
- pelo ADMIN.

Não exige documento ou formulário adicional.

Registrar:
- aluno;
- docente;
- responsável pela criação;
- data de início;
- data de encerramento;
- estado.

Regra inicial: um aluno terá um orientador principal ativo por vez. Preservar histórico de vínculos anteriores. Não apagar registros ao remover ou trocar orientação.

## 7.2 Seleção pelo aluno

Se o aluno não tiver orientador, poderá escolher um DOCENTE disponível para orientação.

Se já possuir orientador, não substituir silenciosamente. A troca deve ser explícita e registrada.

## 7.3 Ações do docente

O DOCENTE poderá:
- consultar lista de orientandos;
- pesquisar por nome ou matrícula;
- abrir ficha do orientando;
- visualizar título, resumo e arquivos dos trabalhos;
- vincular aluno sem orientador;
- encerrar vínculo;
- designar trabalho para evento futuro;
- designar avaliadores;
- cadastrar critérios extras;
- acompanhar avaliações e assinaturas;
- baixar a declaração antiplágio assinada.

## 7.4 Remoção de orientação

O docente poderá encerrar o vínculo com seu orientando. Solicitar confirmação e registrar data, responsável e motivo opcional. Não apagar histórico nem trabalhos já submetidos.

# 8. Trabalhos de conclusão

## 8.1 Submissão pelo aluno

O aluno terá uma opção clara `ENVIAR MEU TCC`.

Campos:
- título;
- resumo;
- link do Overleaf;
- PDF do artigo;
- PDF da apresentação;
- evento, quando já associado ou disponível;
- orientador, caso ainda não exista vínculo.

O artigo e a apresentação poderão ser enviados em momentos diferentes. O sistema não pode exigir que ambos sejam enviados simultaneamente.

O título e o resumo serão obrigatórios para uma submissão completa. O link do Overleaf será opcional, salvo decisão institucional posterior.

## 8.2 Arquivos e versões

Aceitar inicialmente PDF para:
- artigo;
- apresentação;
- declaração antiplágio assinada;
- ATA assinada.

Validar tamanho, MIME type e assinatura real do arquivo. Não confiar somente na extensão.

Armazenar:
- nome original;
- nome interno aleatório;
- tipo;
- tamanho;
- hash SHA-256;
- versão;
- usuário que enviou;
- data/hora.

Não sobrescrever arquivos sem registrar a operação. Manter versões de artigos e apresentações quando forem substituídos, pois o histórico de submissão é relevante.

Arquivos privados nunca devem ficar em diretórios públicos.

## 8.3 Situação do trabalho

Estados possíveis:
- rascunho;
- submetido;
- aguardando avaliadores;
- aguardando avaliações;
- avaliações concluídas;
- ATA aguardando geração;
- ATA gerada;
- assinaturas em andamento;
- ATA finalizada;
- documentação pendente;
- concluído;
- cancelado.

As transições devem ser controladas pelo backend.

# 9. Designação da banca avaliadora

O DOCENTE orientador poderá designar os demais docentes avaliadores para cada trabalho.

Regras:
- O orientador integra a banca e avalia o trabalho.
- A configuração usual será orientador + dois docentes examinadores.
- O sistema deverá aceitar quantidade variável de avaliadores, não fixar três no banco.
- Impedir duplicação do mesmo docente na mesma banca.
- Registrar ordem dos participantes.
- Cada avaliador verá e avaliará somente trabalhos para os quais foi designado, além dos trabalhos de seus próprios orientandos.
- A ATA deverá identificar o orientador como `Professor Orientador` e os demais como `Examinador 1`, `Examinador 2` etc.
- Todas as avaliações obrigatórias devem estar concluídas antes da geração da ATA.

# 10. Critérios e avaliação

## 10.1 Critérios gerais

O banco deve permitir múltiplos critérios, embora inicialmente seja cadastrado somente o critério geral definido pela instituição.

Cada critério terá:
- nome;
- descrição;
- nota mínima;
- nota máxima;
- ativo/inativo;
- ordem de exibição.

**Não implementar pesos.** Não existe ponderação entre critérios.

O nome e a descrição do critério inicial deverão ser carregados conforme informação confirmada pelo responsável. Todos os critérios, sem exceção, devem usar escala de **0 (zero) a 100 (cem)**. Não inventar o nome ou a descrição do critério inicial.

## 10.2 Critérios extras

O DOCENTE poderá cadastrar critérios extras específicos para um trabalho sob sua orientação.

Esses critérios:
- aplicam-se somente ao trabalho correspondente;
- não alteram os critérios gerais;
- aparecem para todos os avaliadores daquele trabalho;
- devem ser preservados como fotografia histórica da avaliação;
- não possuem pesos;
- obrigatoriamente usam escala de **0 (zero) a 100 (cem)**, assim como os critérios gerais.

## 10.3 Tela de avaliação

A avaliação deve ser simples e rápida:

1. DOCENTE clica no botão destacado `AVALIAR`.
2. O sistema apresenta trabalhos pendentes destinados a ele.
3. Seleciona um trabalho.
4. Visualiza título, resumo e acesso ao artigo.
5. Preenche as notas de todos os critérios em uma única tela.
6. Confere o resultado calculado.
7. Confirma a avaliação.

Exibir escala e descrição de cada critério. Validar os limites. Permitir rascunho, se necessário. Depois de finalizada, uma avaliação só poderá ser reaberta por procedimento autorizado e auditado.

## 10.4 Cálculo das notas — regra definida

Não utilizar pesos em nenhuma etapa.

Cada critério, geral ou extra, deve ser avaliado individualmente em uma escala de **0 (zero) a 100 (cem)**, inclusive. O sistema deve rejeitar valores inferiores a zero ou superiores a 100.

Quando houver um único critério, a nota do avaliador será a nota atribuída a esse critério.

Quando houver mais de um critério, a nota final daquele avaliador será a **média aritmética simples** das notas de todos os critérios aplicáveis àquele trabalho:

\[
N_{avaliador}=\frac{C_1+C_2+\cdots+C_n}{n}
\]

Não somar os critérios. Não aplicar pesos. Não utilizar média ponderada.

A nota final do trabalho será a média aritmética simples das notas finais de todos os avaliadores obrigatórios, incluindo o orientador:

\[
N_{trabalho}=\frac{N_1+N_2+\cdots+N_m}{m}
\]

Manter os resultados na escala de 0 a 100 e adotar regra consistente de precisão e arredondamento. A regra de arredondamento deve ser documentada e aplicada igualmente na interface, no banco e na ATA.

Armazenar notas individuais, resultado de cada avaliador, nota final e versão da regra de cálculo utilizada. Alterações futuras nos critérios não podem alterar avaliações históricas.

# 11. Declaração antiplágio

## 11.1 Geração automática

O aluno poderá clicar em `GERAR DECLARAÇÃO ANTIPLÁGIO`.

O backend colocará a tarefa na fila e gerará automaticamente o PDF A4 usando XeLaTeX dentro do Docker.

O documento deverá conter:
- nome completo;
- matrícula;
- demais dados previstos no modelo;
- cidade;
- data por extenso em português;
- texto institucional;
- espaço para assinatura.

O aluno baixa o PDF, assina externamente e envia o PDF assinado.

## 11.2 Acesso

- Aluno acessa seu próprio documento.
- DOCENTE orientador acessa a declaração assinada de seus orientandos.
- Outros docentes não acessam.
- Público não acessa.
- ADMIN não recebe acesso automático ao conteúdo do documento, salvo regra institucional excepcional.

## 11.3 Modelo

Usar o arquivo `declaração-guilherme.tex` e o PDF de referência como base, preservando o texto jurídico e institucional.

Não reescrever o conteúdo legal sem autorização.

A data deve ser automatizada. A regra sobre data de emissão ou assinatura está listada como ponto de confirmação.

# 12. ATA de defesa

## 12.1 Geração automática

A ATA será gerada automaticamente em PDF A4, por XeLaTeX dentro do Docker.

O sistema deverá gerar a ATA depois que todas as avaliações obrigatórias forem concluídas.

Dados:
- data da apresentação, obtida exclusivamente do evento atual associado ao trabalho;
- horário da apresentação, obtido exclusivamente do evento atual associado ao trabalho;
- local cadastrado para esse evento;
- nome completo do aluno;
- matrícula;
- título do trabalho;
- nome do orientador;
- nomes dos examinadores;
- nota final;
- linhas de assinatura;
- texto institucional do modelo.

A nota final será a média aritmética das notas finais dos avaliadores, incluindo o orientador.

### Fonte obrigatória da data, horário e local da ATA

A data e o horário impressos na ATA **NUNCA** serão a data ou o horário de geração do PDF, nem a data ou o horário do momento em que alguém iniciou ou assinou o processo.

A fonte será sempre o evento cadastrado no sistema ao qual aquele trabalho está atualmente vinculado:
- usar a data de apresentação cadastrada no evento atual;
- usar o horário de apresentação cadastrado no evento atual;
- usar o local cadastrado no evento atual.

Se existirem vários eventos no sistema, não escolher simplesmente o evento mais recente por data de cadastro. Deve ser utilizado o evento atual/futuro ao qual o trabalho está associado para aquela apresentação — isto é, o evento cuja apresentação ainda não aconteceu. Se o trabalho tiver mais de uma associação futura, o sistema deverá exigir que uma delas seja explicitamente identificada como evento atual antes de gerar a ATA. Não inferir nem usar a data de geração como alternativa.

A data e o horário do evento devem permanecer como referência da ATA mesmo que o PDF seja gerado novamente dias depois ou que as assinaturas ocorram posteriormente. A data/hora real de geração e de cada assinatura podem ser guardadas internamente para auditoria, mas nunca substituem a data e o horário da apresentação impressos na ATA.

## 12.2 Estrutura

- Orientador identificado como `Professor Orientador`.
- Demais docentes identificados como `Examinador 1`, `Examinador 2` e assim sucessivamente.
- Linha acima de cada identificação para assinatura.
- Campo do aluno identificado como `Aluno: Nome completo`, com linha de assinatura acima.

Na primeira página:
- opção `( ) Publicação do artigo intitulado...` permanece em branco;
- opção `(X) Defesa em sessão pública realizada às...` deve ser marcada e preenchida;
- inserir horário, local, título e demais dados previstos no modelo.

Na página de deliberação:
- inserir nota final;
- preservar texto e campos do modelo;
- apresentar as assinaturas dos participantes;
- incluir assinatura do aluno.

Aceitar quantidade variável de examinadores, com paginação automática e sem cortes.

## 12.3 Privacidade e disponibilidade

A ATA individual contém nota e dados acadêmicos sigilosos.

A ATA final ficará disponível exclusivamente:
- ao aluno avaliado;
- ao DOCENTE orientador daquele aluno;
- aos DOCENTES que participaram como avaliadores daquela banca.

**Nunca disponibilizar a ATA a outros alunos. Nunca disponibilizar a outros docentes que não participaram daquela banca. Nunca publicar a ATA na página do evento ou nos anais.**

O perfil APRESENTADOR não dá acesso à ATA.

O ADMIN não recebe acesso automático à ATA individual apenas por ser ADMIN.

# 13. Fluxo sequencial de assinaturas da ATA

Implementar fluxo sequencial controlado pelo backend **individual para cada trabalho/ATA**. Nunca criar uma fila global de assinaturas compartilhada entre trabalhos.

Cada trabalho terá sua própria ATA e seu próprio processo de assinatura. Um DOCENTE poderá participar de vários processos de assinatura simultaneamente, um para cada trabalho em que seja orientador ou avaliador. A posição e o estado de assinatura em um trabalho não podem bloquear, alterar ou interferir no fluxo de outro trabalho.

## 13.1 Regras gerais e início pelo aluno

1. Após todas as avaliações obrigatórias, a ATA é gerada automaticamente ou por ação explícita autorizada.
2. A ordem dos signatários é definida e registrada antes do início.
3. **O aluno avaliado é o primeiro signatário e deve iniciar o processo de assinatura da ATA do próprio trabalho.**
4. O aluno autenticado verá somente a ATA e o processo de assinatura dos seus próprios trabalhos. Não terá acesso às ATAs de outros alunos.
5. O aluno baixa a ATA, assina externamente e envia o PDF assinado pelo próprio sistema.
6. O backend valida o arquivo, registra o envio e libera o próximo signatário docente.
7. Depois do aluno, os DOCENTES assinam sequencialmente na ordem previamente definida para aquela banca.
8. Cada DOCENTE só poderá baixar e enviar a ATA daquele trabalho quando chegar sua vez.
9. Os demais signatários visualizarão que estão aguardando.
10. O processo termina quando o aluno e todos os DOCENTES signatários obrigatórios tiverem enviado suas assinaturas.
11. A ATA final fica disponível ao aluno avaliado e aos DOCENTES participantes daquela banca, conforme as regras de privacidade desta especificação.

Não permitir pular signatários nem enviar fora da ordem.

## 13.2 Visibilidade e experiência do aluno

O aluno terá, na área do seu próprio trabalho, uma ação como `ASSINAR ATA`.

Essa ação:
- aparecerá somente quando a ATA estiver gerada e pronta para assinatura;
- permitirá baixar a ATA original gerada pelo sistema;
- permitirá enviar o PDF após a assinatura do aluno;
- mostrará o estado `Aguardando sua assinatura`, `Aguardando assinaturas dos docentes` ou `ATA finalizada`;
- não mostrará outros trabalhos, atas ou processos de assinatura de outros alunos.

O aluno não poderá iniciar, consultar ou interferir no fluxo de assinatura de qualquer outro trabalho.

## 13.3 Visibilidade e experiência do DOCENTE

O DOCENTE terá uma área `ATAs para assinar`, com uma lista de trabalhos nos quais participa como orientador ou avaliador e cuja assinatura esteja pendente.

A lista deverá mostrar:
- identificação do trabalho;
- aluno;
- evento;
- situação da assinatura;
- se já chegou a vez daquele docente.

O DOCENTE poderá ter vários trabalhos nessa lista. Cada linha representa um processo independente.

O docente somente poderá baixar e enviar o PDF quando:
- estiver cadastrado como signatário daquela ATA;
- o processo daquele trabalho estiver aguardando sua assinatura;
- todos os signatários anteriores daquele mesmo trabalho tiverem concluído.

A assinatura ou o envio de um documento em um trabalho não deve alterar a situação de outro trabalho.

## 13.4 Ordem dos signatários

A ordem de assinatura de cada ATA será:
1. aluno avaliado;
2. orientador;
3. examinadores, na ordem definida na composição da banca.

A ordem deve ser armazenada individualmente para cada ATA e congelada quando o fluxo for iniciado. Alterações posteriores na composição da banca não podem modificar silenciosamente um processo já iniciado.

Se houver necessidade de corrigir a banca ou reiniciar o fluxo, a operação deverá ser explícita, autorizada e auditada.

## 13.5 Validação e avanço do fluxo

A cada envio:
- verificar se o usuário autenticado é o signatário atual daquela ATA;
- verificar se o arquivo é um PDF válido;
- calcular SHA-256;
- registrar signatário, posição, data/hora e resultado;
- substituir o arquivo assinado atual pelo arquivo recebido;
- avançar exatamente uma posição;
- impedir que duplo clique, reenvio ou requisições concorrentes avancem mais de uma posição.

Se o arquivo for rejeitado, o mesmo signatário continua sendo o responsável pela próxima tentativa.

## 13.6 Versões da ATA: regra simplificada

Não manter histórico de todos os PDFs assinados.

- Deve existir apenas a **última versão assinada** disponível no sistema.
- Quando um novo signatário enviar sua versão, substituir o arquivo assinado anterior pelo novo arquivo.
- Não disponibilizar versões assinadas anteriores para download.
- Preservar somente metadados de auditoria: quem enviou, ordem, data/hora, hash e estado.
- O PDF original gerado pelo LaTeX poderá ser armazenado e regenerado a qualquer momento a partir dos dados e da versão do template.
- O PDF original não deve ser confundido com a última versão assinada.

Ao final, o arquivo disponível para download será a última versão assinada, contendo cumulativamente as assinaturas do aluno e de todos os DOCENTES participantes da banca.

## 13.7 Segurança e integridade

- Verificar se o arquivo enviado é PDF válido.
- Calcular hash SHA-256.
- Registrar signatário e posição, incluindo o aluno como primeiro signatário.
- Bloquear envio por usuário diferente do signatário atual.
- Impedir que uma falha ou reenvio duplicado avance duas posições.
- Em caso de arquivo rejeitado, manter o mesmo signatário como responsável pela próxima tentativa.
- Registrar operações em auditoria.
- Garantir isolamento completo entre processos de assinatura de trabalhos diferentes.

O sistema não deve afirmar que assinatura manuscrita digitalizada equivale a assinatura digital ICP-Brasil. O fluxo atual é de upload sequencial de PDF assinado externamente.

# 14. Geração de PDFs por LaTeX — requisito central

## 14.1 Automação integral

A geração dos PDFs deve ser inteiramente automática.

O usuário clica em um botão no sistema. O backend:
1. valida os dados;
2. cria tarefa persistente no PostgreSQL;
3. worker busca a tarefa;
4. prepara os dados do documento;
5. preenche o template LaTeX;
6. executa XeLaTeX dentro do ambiente Docker;
7. verifica o resultado;
8. armazena o PDF em volume persistente;
9. atualiza o estado da tarefa;
10. libera o download ao usuário autorizado.

Não exigir intervenção humana para compilar o LaTeX.

O servidor hospedeiro não precisa ter LaTeX instalado. As dependências estarão na imagem Docker.

## 14.2 Segurança do LaTeX

Não concatenar texto livre do usuário diretamente como comandos LaTeX.

Implementar escaping rigoroso de caracteres especiais, incluindo `%`, `$`, `&`, `#`, `_`, `{`, `}`, `~`, `^` e barra invertida.

Não permitir que título, resumo, nome ou outros campos submetidos pelo usuário executem comandos LaTeX.

Executar compilação em diretório temporário isolado, sem acesso desnecessário ao sistema de arquivos e sem shell escape, salvo necessidade técnica justificada e segura.

## 14.3 Cabeçalho institucional

O cabeçalho deverá aparecer em todas as páginas.

Ordem e conteúdo:

1. Brasão da República;
2. `MINISTÉRIO DA EDUCAÇÃO`;
3. `SECRETARIA DE EDUCAÇÃO PROFISSIONAL E TECNOLÓGICA`;
4. `INSTITUTO FEDERAL DE EDUCAÇÃO, CIÊNCIA E TECNOLOGIA DE MINAS GERAIS`;
5. `CAMPUS OURO BRANCO`;
6. `Av. Afonso Sardinha, nº 90, Bairro Pioneiros, CEP: 36420-000, Ouro Branco - Minas Gerais`;
7. `(31) 3742-2149 - gabinete.ourobranco@ifmg.edu.br`.

Preservar a capitalização indicada. Não converter o endereço inteiro para maiúsculas.

O cabeçalho deve começar aproximadamente a 0,5 cm do topo da folha A4, conforme o modelo institucional. O brasão e o cabeçalho não devem ser empurrados para baixo por uma margem superior convencional de 2,5 cm.

O corpo começa abaixo do cabeçalho, com espaçamento compatível com o modelo.

Cada página deverá repetir o cabeçalho.

## 14.4 Qualidade visual

- Papel A4.
- Brasão com proporção preservada.
- Fontes e tamanhos consistentes com os PDFs de referência.
- Cabeçalho repetido em todas as páginas.
- Acentos e caracteres portugueses corretos.
- Nenhum texto cortado.
- Nenhuma assinatura dividida inadequadamente.
- Campos de assinatura com espaço suficiente.
- Quebras de página controladas.
- Nomes e títulos longos tratados corretamente.
- Número variável de avaliadores sem quebra de layout.

A simples existência de um arquivo PDF não significa que a geração foi bem-sucedida. Implementar validação técnica e testes visuais.

## 14.5 Templates e arquivos de referência

Manter templates versionados em diretório próprio, por exemplo:

`backend/templates/documents/ata/`  
`backend/templates/documents/declaracao-antiplagio/`

Manter também:
- imagem oficial do brasão;
- fontes utilizadas;
- arquivos auxiliares;
- documentação das variáveis;
- versão dos modelos.

Os arquivos `.tex`, PDFs e imagem do brasão fornecidos pelo responsável são a referência visual e textual. O agente deve inspecioná-los antes de implementar o template definitivo.

Não alterar o texto institucional ou jurídico sem autorização.

# 15. Consolidação de apresentações e ANAIS

## 15.1 PDF consolidado das apresentações

O DOCENTE designado como APRESENTADOR terá botão destacado:

`GERAR PDF COM TODAS AS APRESENTAÇÕES`

O sistema deverá:
- localizar apresentações dos trabalhos associados ao evento;
- respeitar a ordem da programação;
- juntar os PDFs em um único arquivo;
- preservar orientação, tamanho de página e qualidade;
- registrar a versão da compilação;
- apresentar relatório de arquivos incluídos e pendentes;
- permitir conferência antes da publicação.

Não incluir apresentações ausentes, inválidas ou não autorizadas.

## 15.2 Anais do evento

Botão destacado:

`GERAR ANAIS DO EVENTO`

O sistema deverá:
- localizar os artigos finais associados ao evento;
- respeitar a ordem definida;
- juntar os PDFs em um único arquivo;
- preservar o conteúdo original;
- permitir futuramente capa e sumário, caso aprovados;
- registrar os trabalhos incluídos;
- permitir conferência antes da publicação.

Nunca incluir:
- atas;
- notas;
- avaliações;
- declarações antiplágio;
- documentos privados.

## 15.3 Publicação

Os documentos consolidados só serão públicos após ação explícita de publicação pelo DOCENTE designado como APRESENTADOR. **Não exigir aprovação adicional do ADMIN para publicar.**

Estados:
- em preparação;
- gerado;
- aguardando revisão;
- publicado;
- substituído.

Preservar metadados e versões de publicações. A regra de substituição não se confunde com a regra simplificada de versões da ATA.

# 16. Interface e identidade visual

## 16.1 Direção visual

A aplicação deverá utilizar um **tema claro em tons de azul**, com fundo azul muito claro e gradiente suave que se aproxime do branco. Não utilizar uma interface inteiramente branca e preta.

Direção visual:
- fundo geral azul muito claro, com gradiente discreto;
- áreas de conteúdo em branco ou azul quase branco;
- cartões brancos com bordas suaves;
- azul médio/escuro para ações principais;
- azul claro para destaques e áreas informativas;
- textos em azul-marinho ou cinza-azulado escuro;
- cores de sucesso, alerta e erro apenas para estados semânticos;
- sombras leves e discretas;
- aparência institucional, moderna, limpa e acolhedora.

Sugestão inicial de tokens de cor, ajustáveis:
- fundo superior: `#EAF4FF`;
- fundo inferior: `#F7FBFF`;
- superfície/cartão: `#FFFFFF`;
- superfície secundária: `#F2F8FF`;
- azul primário: `#2367A8`;
- azul escuro: `#17466F`;
- azul de destaque: `#78B7E8`;
- texto principal: `#18324A`;
- texto secundário: `#587086`;
- borda: `#D8E7F5`.

Usar gradiente leve no fundo principal, por exemplo de azul claro no topo/lateral para quase branco. Evitar gradientes fortes, excesso de saturação ou aparência infantil.

## 16.2 Componentes e navegação

Utilizar Material UI com tema personalizado.

A interface deve ter:
- menu lateral ou superior claro;
- identificação do usuário e contexto atual;
- painel inicial por perfil;
- cartões de resumo;
- tabelas com busca, filtro e ordenação;
- indicadores de status;
- botões grandes e visíveis para ações principais;
- tooltips em campos e ações que precisem de explicação;
- mensagens claras de sucesso e erro;
- confirmação para ações destrutivas;
- estados de carregamento e processamento.

## 16.3 Redundância intencional de ações

As ações principais devem aparecer tanto no menu quanto na tela inicial e no contexto do objeto.

Exemplos:
- `AVALIAR` no menu DOCENTE, no painel inicial e na lista de avaliações pendentes;
- `MEUS ORIENTANDOS` no menu e no painel;
- `CRIAR EVENTO` no menu ADMIN e em destaque na página inicial;
- `ENVIAR MEU TCC` no menu ALUNO e em botão central;
- `GERAR ANAIS` no painel do APRESENTADOR e na página do evento.

Não esconder ações frequentes em menus de três pontos.

## 16.4 Painel ADMIN

Exibir:
- eventos ativos e próximos;
- botão `CRIAR EVENTO`;
- docentes aguardando aprovação;
- usuários cadastrados;
- trabalhos por evento;
- tarefas de documentos em processamento;
- falhas de geração;
- atalhos administrativos.

Não misturar funções docentes no contexto ADMIN.

## 16.5 Painel DOCENTE

Exibir:
- botão `AVALIAR`;
- avaliações pendentes;
- avaliações concluídas;
- ATAs aguardando sua assinatura;
- aviso de quando é sua vez;
- área `MEUS ORIENTANDOS`;
- trabalhos dos orientandos;
- pendências documentais dos orientandos;
- atalhos para designar avaliadores e associar trabalho a evento.

As áreas de avaliação e orientação devem coexistir naturalmente, sem seleção de perfil.

## 16.6 Painel ALUNO

Exibir:
- botão `ENVIAR MEU TCC`;
- botão `ENVIAR APRESENTAÇÃO`;
- botão `GERAR DECLARAÇÃO ANTIPLÁGIO`;
- botão `ENVIAR DECLARAÇÃO ASSINADA`;
- orientador;
- evento associado;
- situação do trabalho;
- histórico de arquivos;
- próximos passos.

## 16.7 Painel APRESENTADOR

Exibir:
- evento sob responsabilidade;
- botão `GERAR PDF DAS APRESENTAÇÕES`;
- botão `GERAR ANAIS`;
- situação dos documentos;
- trabalhos incluídos;
- pendências e erros;
- botão de publicação após revisão.

## 16.8 Responsividade e acessibilidade

Otimizar prioritariamente para desktop, mas permitir uso em tablets e celulares.

- Labels associados aos campos.
- Tooltips acessíveis por mouse e teclado.
- Contraste adequado.
- Navegação por teclado.
- Foco visível.
- Mensagens de validação junto ao campo.
- Não depender exclusivamente de cores para indicar estado.

# 17. Modelo de dados — entidades mínimas

Utilizar PostgreSQL relacional, com migrations versionadas, chaves estrangeiras, índices e restrições.

O agente pode ajustar a normalização, mas não pode eliminar histórico, integridade ou controle de acesso.

## 17.1 Usuários e perfis

### `users`
- `id` UUID;
- `full_name`;
- `email` único;
- `phone`;
- `institutional_id`;
- `institutional_id_type`;
- `password_hash`;
- `status`;
- `created_at`;
- `updated_at`.

### `user_roles`
- `user_id`;
- `role_code` (ADMIN, DOCENTE ou ALUNO);
- `granted_by`;
- `granted_at`.

Restrições:
- ALUNO é exclusivo.
- ADMIN pode coexistir com DOCENTE.
- DOCENTE não se divide em professor e orientador.
- APRESENTADOR é atribuição por evento, não perfil global independente.

### `faculty_profiles`
- `user_id`;
- `siape`;
- `approval_status`;
- `approved_by`;
- `approved_at`.

### `student_profiles`
- `user_id`;
- `academic_registration`;
- `course`;
- outros campos acadêmicos opcionais.

### `account_tokens`
Para primeiro acesso e redefinição de senha, se utilizados:
- usuário;
- hash do token;
- tipo;
- validade;
- data de uso.

### `audit_logs`
- ator;
- ação;
- entidade;
- identificador;
- valores relevantes antes/depois, sem senhas ou segredos;
- data/hora;
- informações técnicas pertinentes.

## 17.2 Orientação

### `advisor_relationships`
- aluno;
- docente;
- criado por;
- início;
- fim;
- situação.

Preservar histórico e garantir um orientador principal ativo por aluno, conforme regra inicial.

## 17.3 Eventos

### `events`
- nome;
- edição/ano;
- descrição;
- datas;
- horários;
- local;
- estado;
- prazos;
- dados de publicação;
- criado por.

### `event_presenters`
- evento;
- docente;
- atribuído por;
- data.

## 17.4 Trabalhos e arquivos

### `works`
- aluno;
- orientador;
- evento;
- título;
- resumo;
- link Overleaf;
- estado;
- data de submissão;
- data/horário/local da apresentação;
- versão atual.

### `work_files`
- trabalho;
- tipo;
- nome original;
- chave de armazenamento;
- MIME;
- tamanho;
- SHA-256;
- versão;
- enviado por;
- data.

### `work_status_history`
- trabalho;
- estado anterior;
- novo estado;
- ator;
- data;
- observação.

## 17.5 Banca e avaliação

### `work_evaluators`
- trabalho;
- docente;
- papel (advisor/examiner);
- ordem;
- obrigatório;
- situação.

### `evaluation_criteria`
- nome;
- descrição;
- nota mínima;
- nota máxima;
- ativo;
- ordem.

### `work_criteria`
- trabalho;
- critério geral ou extra;
- nome e descrição congelados;
- escala;
- origem;
- ordem.

### `evaluations`
- trabalho;
- avaliador;
- estado;
- nota calculada;
- enviado em;
- versão da fórmula.

### `evaluation_scores`
- avaliação;
- critério;
- nota;
- comentário opcional.

Não incluir coluna de peso.

## 17.6 ATA e assinaturas

### `defense_minutes`
- trabalho;
- número/versão lógica;
- estado;
- nota final congelada;
- versão do template;
- caminho do PDF original LaTeX;
- hash do original;
- caminho da última versão assinada;
- hash da última versão assinada;
- gerado por;
- gerado em.

### `minute_signatures`
- ATA;
- signatário (aluno ou docente);
- tipo do signatário (student/advisor/examiner);
- ordem;
- estado;
- data/hora do envio;
- hash do arquivo enviado;
- observação.

A sequência é específica de cada ATA/trabalho e começa pelo aluno. Não criar uma fila global. Não manter os PDFs assinados anteriores; manter somente o arquivo assinado atual e os metadados de auditoria.

## 17.7 Fila e documentos

### `document_jobs`
- tipo;
- entidade relacionada;
- estado;
- progresso;
- tentativas;
- erro;
- solicitado por;
- criado em;
- iniciado em;
- concluído em.

### `generated_documents`
- tipo;
- entidade;
- versão;
- caminho;
- hash;
- versão do template;
- gerado por;
- data;
- estado de publicação, quando aplicável.

## 17.8 Publicações

### `event_publications`
- evento;
- tipo (proceedings, presentations);
- versão;
- arquivo;
- hash;
- estado;
- publicado por;
- publicado em.

### `event_publication_items`
- publicação;
- trabalho;
- arquivo/versão utilizada;
- posição;
- resultado da inclusão.

# 18. API e organização do backend

Criar API REST versionada, por exemplo `/api/v1`.

Módulos:
- auth;
- users;
- roles/permissions;
- faculty;
- students;
- advisors;
- events;
- works;
- files;
- evaluators;
- criteria;
- evaluations;
- minutes;
- signatures;
- documents;
- publications;
- audit;
- health.

Requisitos:
- validação no backend;
- erros em formato consistente;
- paginação;
- filtros;
- ordenação;
- limites de upload;
- autenticação;
- autorização por recurso;
- transações para operações compostas;
- idempotência para geração e publicação;
- logs estruturados;
- documentação OpenAPI.

Não retornar caminhos físicos do servidor ao frontend. Retornar identificadores e endpoints de download com autorização.

# 19. Segurança e privacidade

Implementar:
- HTTPS em produção;
- cookies seguros HttpOnly e SameSite, se utilizada autenticação por sessão;
- proteção CSRF conforme estratégia;
- CORS restrito;
- rate limiting;
- proteção contra força bruta;
- validação de uploads;
- controle de acesso por objeto;
- prevenção de IDOR;
- proteção contra injeção SQL;
- hash seguro de senhas;
- tokens temporários armazenados como hash;
- secrets por variáveis de ambiente;
- auditoria de operações sensíveis;
- backup e restauração;
- princípio do menor privilégio.

Documentos acadêmicos não podem ficar disponíveis em diretórios públicos. Downloads passam pelo backend com autorização.

# 20. Desempenho e confiabilidade

O volume esperado é pequeno. Priorizar simplicidade, robustez e rapidez.

- Consultas paginadas no servidor.
- Índices em e-mail, matrícula, evento, aluno, orientador, estado e relacionamentos usados em filtros.
- Upload e download com streaming quando necessário.
- Geração de PDFs em background.
- Atualização de status no frontend.
- Evitar duplicação de jobs por duplo clique.
- Transações em operações críticas.
- Recuperação de tarefas após reinicialização.
- Health checks para API e banco.
- Não criar cache ou infraestrutura distribuída sem necessidade real.

# 21. Seed e dados iniciais

Criar seed idempotente com:
- usuário ADMIN inicial para o responsável indicado;
- docentes informados;
- critério geral de avaliação, após confirmação de nome e escala;
- dados institucionais do IFMG Campus Ouro Branco.

Docentes informados:
- Angelo Magno de Jesus — angelo.jesus@ifmg.edu.br
- Charles Tim Batista Garrocho — charles.garrocho@ifmg.edu.br — SIAPE informado: 1146652
- Daniela Costa Terra — daniela.terra@ifmg.edu.br
- Edilus de Carvalho Castro Penido — edilus.penido@ifmg.edu.br
- Janio Rosa da Silva — janio.silva@ifmg.edu.br — SIAPE informado: 1752938
- Luciana Lourdes Silva — luciana.lourdes.silva@ifmg.edu.br
- Suelen Mapa de Paula — suelen.mapa@ifmg.edu.br
- Ederson Naves Fernandes Gonçalves Júnior — ederson.junior@ifmg.edu.br

Não inventar SIAPEs ausentes. Campos não fornecidos devem permanecer pendentes de conferência.

O ADMIN inicial será Janio Rosa da Silva, conforme indicação do responsável. Sua senha permanente será definida no primeiro acesso.

A carga inicial deve ser idempotente e não sobrescrever informações posteriormente alteradas.

# 22. Documentação obrigatória

Criar pasta `DOC/` na raiz do projeto.

### `project.md`
Título: `O que é este projeto`. Explicar objetivo, escopo, perfis, funcionalidades e tecnologias.

### `running.md`
Getting Started:
- requisitos;
- versões;
- instalação;
- `.env`;
- migrations;
- seed;
- Docker Compose;
- execução local;
- testes;
- geração automática de documentos;
- backup e restauração.

### `architecture.md`
Arquitetura, módulos, linguagens, containers, fluxo de documentos, estrutura de pastas e decisões técnicas.

### `database.md`
Todas as tabelas, colunas, tipos, chaves, índices, relacionamentos e regras de integridade.

### `readme.md`
Visão geral acessível a todos, sem excesso de complexidade técnica. Não mencionar tecnologias que não são utilizadas.

### Documentos adicionais recomendados
- `business-rules.md`;
- `permissions.md`;
- `documents.md`;
- `api.md`;
- `deployment.md`;
- `decisions.md`.

A documentação deve refletir o código efetivamente implementado.

# 23. Testes obrigatórios

Criar testes unitários, de integração e ponta a ponta.

## Identidade e perfis
- ALUNO não pode acumular outro perfil.
- ADMIN pode coexistir com DOCENTE.
- ADMIN + DOCENTE recebe escolha de contexto após login.
- DOCENTE não precisa alternar entre professor e orientador.
- DOCENTE sem orientandos continua podendo avaliar.
- DOCENTE vê seus orientandos e os trabalhos destinados a ele.
- DOCENTE não vê trabalhos de outros avaliadores.
- DOCENTE pendente não acessa funções docentes.
- APRESENTADOR não acessa ATAs individuais.

## Cadastro
- aluno cadastra e entra sem aprovação;
- docente cadastra e fica pendente;
- somente ADMIN aprova DOCENTE;
- e-mail é validado sintaticamente, sem envio de confirmação;
- senha nunca é armazenada em texto puro;
- primeiro acesso do ADMIN exige definição de senha permanente.

## Orientação
- aluno sem orientador pode escolher um;
- docente pode vincular aluno sem orientador;
- ADMIN pode vincular;
- troca encerra vínculo anterior e preserva histórico;
- remoção não apaga histórico.

## Eventos e trabalhos
- trabalho não pode ser associado a evento passado;
- trabalho pode ser associado a evento futuro;
- artigo e apresentação podem ser enviados separadamente;
- histórico de artigos e apresentações é preservado.

## Avaliação
- somente avaliadores designados avaliam;
- orientador avalia como DOCENTE participante da banca;
- critérios extras aparecem apenas no trabalho correspondente;
- cálculo sem pesos;
- nota final é média aritmética das notas dos avaliadores;
- avaliação pendente impede ATA.

## Assinaturas
- aluno avaliado é o primeiro signatário e inicia o processo da própria ATA;
- aluno não vê ATAs de outros alunos;
- somente signatário da vez envia;
- signatário posterior não antecipa;
- cada trabalho possui fluxo de assinatura independente;
- docente pode participar de vários fluxos simultâneos sem interferência entre eles;
- arquivo atual é substituído a cada envio válido;
- versões assinadas anteriores não ficam disponíveis;
- metadados de auditoria permanecem;
- ATA final fica disponível ao aluno e aos docentes participantes, e somente a eles.

## Documentos
- geração automática dentro do Docker;
- não exige comando manual;
- PDF A4;
- cabeçalho repetido;
- brasão com proporção correta;
- acentuação correta;
- textos sem cortes;
- ATA variável em número de examinadores;
- data, horário e local da ATA vêm sempre do evento atual associado ao trabalho, nunca do momento de geração do PDF;
- declaração com data por extenso;
- arquivos privados protegidos;
- layout visual comparado com os PDFs oficiais.

## Anais
- somente trabalhos elegíveis são incluídos;
- ordem correta;
- nenhum documento privado é incluído;
- publicação é explícita e auditável.

## Persistência
- dados permanecem após `docker compose down`;
- dados permanecem após rebuild;
- arquivos persistem após reinício dos containers;
- backup e restauração testados.

# 24. Critérios de aceite

O sistema só estará pronto quando:

1. ADMIN conseguir criar e publicar um evento.
2. Alunos conseguirem se cadastrar e entrar sem aprovação.
3. Docentes conseguirem se cadastrar e aguardar aprovação.
4. Usuário ADMIN + DOCENTE conseguir escolher o contexto no login.
5. DOCENTE conseguir usar avaliação e orientação sem alternar perfis.
6. Docente conseguir vincular alunos.
7. Orientador conseguir associar trabalhos a eventos futuros.
8. Orientador conseguir designar avaliadores.
9. Avaliadores conseguirem avaliar em uma única tela.
10. Nota final estiver calculada corretamente.
11. Aluno conseguir gerar, baixar e enviar declaração antiplágio assinada.
12. ATA for gerada automaticamente por LaTeX dentro do Docker.
13. Aluno iniciar o fluxo sequencial da própria ATA.
14. Fluxo de assinatura ser independente por trabalho, permitindo ao DOCENTE participar de vários processos simultâneos.
15. ATA final estiver acessível somente ao aluno e aos docentes participantes.
15. Apresentador conseguir gerar PDF consolidado das apresentações.
16. Apresentador conseguir gerar anais.
17. Página pública mostrar apenas documentos públicos.
18. Fila assíncrona persistir e recuperar tarefas após reinício.
19. Dados e arquivos persistirem após rebuild e `docker compose down`.
20. Testes críticos passarem.
21. Documentação `DOC/` estiver completa.

# 25. Fases de desenvolvimento

## Fase 1 — Fundação
- estrutura do repositório;
- Docker;
- PostgreSQL;
- migrations;
- autenticação;
- usuários e perfis;
- escolha de contexto ADMIN/DOCENTE;
- auditoria;
- tema visual azul claro;
- layout principal.

## Fase 2 — Eventos e orientação
- cadastro de eventos;
- cadastro e aprovação de docentes;
- cadastro de alunos;
- vínculos de orientação;
- painéis por perfil;
- atribuição de apresentadores.

## Fase 3 — Trabalhos e avaliação
- submissão de artigo e apresentação;
- armazenamento e versões;
- designação de avaliadores;
- critérios gerais e extras;
- avaliação e cálculo de notas.

## Fase 4 — Documentos individuais
- XeLaTeX instalado na imagem Docker;
- fila PostgreSQL;
- worker assíncrono;
- declaração antiplágio;
- ATA;
- testes visuais;
- uploads assinados.

## Fase 5 — Assinaturas sequenciais
- máquina de estados;
- sequência de signatários;
- substituição do arquivo assinado atual;
- metadados de auditoria;
- ATA final.

## Fase 6 — Anais e publicação
- consolidação de apresentações;
- consolidação de artigos;
- página pública;
- revisão e publicação.

## Fase 7 — Qualidade e entrega
- testes E2E;
- segurança;
- desempenho;
- backup e restauração;
- documentação final;
- instruções de implantação.

# 26. Pendências de definição — não inventar

As regras principais de assinatura, data/hora da ATA, cálculo das notas e publicação dos anais estão definidas nesta versão. Restam apenas os seguintes dados ou decisões operacionais:

1. **Nome e descrição do critério inicial:** confirmar o texto do critério geral. A escala está definida: 0 a 100.
2. **Data da declaração antiplágio:** recomendação operacional: usar a data de emissão/geração do documento, pois a data da ATA é uma regra diferente e fixa, proveniente do evento.
3. **Troca de orientador:** regra inicial: docente pode encerrar o vínculo, aluno pode solicitar troca, e toda operação fica registrada.
4. **Coorientador:** nesta versão existe apenas um orientador principal ativo.
5. **Link Overleaf:** tratar como opcional.
6. **SIAPEs não fornecidos:** completar somente com dados confirmados por fonte oficial ou pelo responsável.

A publicação dos anais e do PDF consolidado das apresentações é realizada diretamente pelo DOCENTE designado como APRESENTADOR, sem aprovação adicional do ADMIN.

# 27. Instrução final ao agente

Construa o sistema com foco no uso institucional cotidiano.

O evento deve ser o centro da navegação. O DOCENTE deve conseguir orientar e avaliar sem alternar perfis. O ADMIN que também é DOCENTE deve escolher o contexto ao entrar. O ALUNO deve ter um perfil exclusivo. A geração de documentos deve ocorrer automaticamente dentro do Docker, sem qualquer compilação manual. A fila deve ser assíncrona e persistida no PostgreSQL, sem infraestrutura externa de mensageria.

A ATA deve respeitar rigorosamente o modelo institucional. O aluno avaliado inicia o fluxo sequencial da própria ATA; depois assinam o orientador e os demais examinadores. Cada trabalho possui seu próprio fluxo independente, permitindo que um DOCENTE participe de vários processos ao mesmo tempo. A ATA final fica disponível somente ao aluno avaliado e aos docentes participantes da banca. As versões assinadas anteriores não devem ficar disponíveis; apenas o último PDF assinado será mantido, além do original gerado por LaTeX que poderá ser regenerado. A data, o horário e o local impressos na ATA vêm sempre do evento atual associado ao trabalho, nunca do momento de geração do PDF.

A interface deve usar tons claros de azul, gradiente suave para branco, cartões limpos e ações importantes em botões visíveis, redundantes e fáceis de localizar.

Não deixe botões sem ação, permissões somente no frontend, dados fictícios em produção, arquivos privados expostos ou tarefas críticas sem persistência.

Ao concluir cada fase, informe:
- o que foi implementado;
- quais testes foram executados;
- resultados;
- pendências;
- como executar a versão atual.

**Prioridade máxima: correção das regras de negócio, privacidade dos documentos, confiabilidade da geração automática de PDFs e facilidade de uso.**
