import { FieldValues, get, Path, UseFormReturn } from 'react-hook-form';
import { ErroApi } from '../api/cliente';

// campo: liga um TextField do MUI ao React Hook Form, com mensagem de validação junto ao campo.
export function campo<T extends FieldValues>(form: UseFormReturn<T>, nome: Path<T>) {
  const { ref, ...resto } = form.register(nome);
  const erro = get(form.formState.errors, nome)?.message as string | undefined;
  return { ...resto, inputRef: ref, id: `campo-${String(nome)}`, error: !!erro, helperText: erro };
}

// aplicarErrosServidor copia os erros por campo devolvidos pela API para o formulário.
export function aplicarErrosServidor<T extends FieldValues>(form: UseFormReturn<T>, erro: unknown): boolean {
  if (erro instanceof ErroApi && Object.keys(erro.campos).length > 0) {
    Object.entries(erro.campos).forEach(([nome, mensagem]) => form.setError(nome as Path<T>, { message: mensagem }));
    return true;
  }
  return false;
}
