import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, CardContent, Chip, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, IconButton, List, ListItem, ListItemText, Stack, Switch, TextField } from '@mui/material';
import EditIcon from '@mui/icons-material/Edit';
import AddIcon from '@mui/icons-material/Add';
import { api, mensagemDeErro } from '../../api/cliente';
import { Carregando, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';

interface Criterio { id?: string; nome: string; descricao: string; ativo: boolean; ordem: number; fixo?: boolean }

export function Criterios() {
  const notificar = useNotificar();
  const [editando, setEditando] = useState<Criterio | null>(null);
  const { data, isLoading, refetch } = useQuery({ queryKey: ['admin', 'criterios'], queryFn: () => api.get<Criterio[]>('/admin/criterios') });
  const salvar = async () => {
    if (!editando) return;
    try {
      if (editando.id) await api.put(`/admin/criterios/${editando.id}`, editando);
      else await api.post('/admin/criterios', editando);
      notificar('Critério salvo.');
      setEditando(null);
      void refetch();
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <>
      <Titulo titulo="Critérios gerais de avaliação" acoes={<Button variant="contained" startIcon={<AddIcon />} onClick={() => setEditando({ nome: '', descricao: '', ativo: true, ordem: (data?.length ?? 0) + 1 })}>Novo critério</Button>} />
      <Alert severity="info" sx={{ mb: 3 }}>
        Todos os critérios usam escala de <strong>0 a 100</strong> e <strong>não têm pesos</strong>. A nota de cada avaliador é a média simples dos critérios;
        a nota final é a média simples dos avaliadores (incluindo o orientador), com 2 casas decimais. Aprovação com nota final de <strong>60,00</strong> ou mais. O critério <strong>Nota</strong> é obrigatório e vale para todos os trabalhos. Alterações não afetam trabalhos que já possuem avaliação finalizada.
      </Alert>
      <Card><CardContent>
        {isLoading ? <Carregando /> : !data?.length ? <Vazio>Nenhum critério cadastrado. Cadastre o critério geral confirmado pela instituição.</Vazio> : (
          <List>
            {data.map((c) => (
              <ListItem key={c.id} secondaryAction={<IconButton aria-label="Editar critério" onClick={() => setEditando(c)}><EditIcon /></IconButton>}>
                <ListItemText primary={<Stack direction="row" spacing={1} alignItems="center"><span>{c.ordem}. {c.nome}</span><Chip size="small" label="0 a 100" />{c.fixo && <Chip size="small" color="primary" label="Obrigatório" />}{!c.ativo && <Chip size="small" label="Inativo" variant="outlined" />}</Stack>} secondary={c.descricao} />
              </ListItem>
            ))}
          </List>
        )}
      </CardContent></Card>
      <Dialog open={!!editando} onClose={() => setEditando(null)} maxWidth="sm" fullWidth>
        <DialogTitle>{editando?.id ? 'Editar critério' : 'Novo critério'}</DialogTitle>
        {editando && (
          <DialogContent>
            <Stack spacing={2} sx={{ mt: 1 }}>
              <TextField label="Nome" disabled={editando.fixo} helperText={editando.fixo ? 'Critério obrigatório: não pode ser renomeado nem desativado.' : undefined} value={editando.nome} onChange={(e) => setEditando({ ...editando, nome: e.target.value })} />
              <TextField label="Descrição" multiline minRows={3} value={editando.descricao} onChange={(e) => setEditando({ ...editando, descricao: e.target.value })} />
              <TextField label="Ordem de exibição" type="number" value={editando.ordem} onChange={(e) => setEditando({ ...editando, ordem: Number(e.target.value) })} />
              <FormControlLabel control={<Switch disabled={editando.fixo} checked={editando.ativo} onChange={(_, v) => setEditando({ ...editando, ativo: v })} />} label="Ativo" />
            </Stack>
          </DialogContent>
        )}
        <DialogActions>
          <Button onClick={() => setEditando(null)}>Cancelar</Button>
          <Button variant="contained" onClick={() => void salvar()}>Salvar</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
