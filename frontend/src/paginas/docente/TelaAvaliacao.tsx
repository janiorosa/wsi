import { useNavigate, useParams } from 'react-router-dom';
import { Button } from '@mui/material';
import { Titulo } from '../../componentes/Comuns';
import { FormularioAvaliacao } from './FormularioAvaliacao';

// Tela completa de avaliação de um trabalho (notas por critério, comentários e parecer, com salvamento automático).
export function TelaAvaliacao() {
  const { trabalhoId } = useParams();
  const navegar = useNavigate();
  return (
    <>
      <Titulo titulo="Avaliação do trabalho" subtitulo="As notas e o parecer são salvos automaticamente enquanto você digita."
        acoes={<Button onClick={() => navegar('/docente')}>Voltar ao painel</Button>} />
      <FormularioAvaliacao trabalhoId={trabalhoId!} />
    </>
  );
}
