import { useRef, useState } from 'react';
import { Button, ButtonProps, CircularProgress } from '@mui/material';
import UploadFileIcon from '@mui/icons-material/UploadFile';
import { enviarArquivo, mensagemDeErro } from '../api/cliente';
import { useNotificar } from './Notificacoes';

const LIMITE_MB = 30;

interface Props extends Omit<ButtonProps, 'onClick' | 'onError'> {
  caminho: string;
  campos?: Record<string, string>;
  rotulo: string;
  aoConcluir?: (resposta: unknown) => void;
  mensagemSucesso?: string;
}

// Botão que abre a seleção de arquivo e envia um PDF. A validação real é feita no servidor.
export function EnvioPDF({ caminho, campos, rotulo, aoConcluir, mensagemSucesso, ...props }: Props) {
  const entrada = useRef<HTMLInputElement>(null);
  const [enviando, setEnviando] = useState(false);
  const notificar = useNotificar();

  const aoSelecionar = async (arquivo: File | undefined) => {
    if (!arquivo) return;
    if (arquivo.size > LIMITE_MB * 1024 * 1024) {
      notificar(`O arquivo excede ${LIMITE_MB} MB.`, 'error');
      return;
    }
    if (arquivo.type && arquivo.type !== 'application/pdf') {
      notificar('Selecione um arquivo PDF.', 'error');
      return;
    }
    setEnviando(true);
    try {
      const resposta = await enviarArquivo(caminho, arquivo, campos);
      notificar(mensagemSucesso ?? 'Arquivo enviado com sucesso.');
      aoConcluir?.(resposta);
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    } finally {
      setEnviando(false);
      if (entrada.current) entrada.current.value = '';
    }
  };

  return (
    <>
      <input
        ref={entrada}
        type="file"
        accept="application/pdf,.pdf"
        hidden
        aria-label={rotulo}
        onChange={(e) => void aoSelecionar(e.target.files?.[0])}
      />
      <Button
        variant="contained"
        {...props}
        startIcon={enviando ? <CircularProgress size={18} color="inherit" /> : <UploadFileIcon />}
        disabled={enviando || props.disabled}
        onClick={() => entrada.current?.click()}
      >
        {enviando ? 'Enviando...' : rotulo}
      </Button>
    </>
  );
}
