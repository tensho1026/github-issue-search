import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { accountErrorPresentation } from "../model/account-errors";

export function AccountRequestAlert({ error }: { error: unknown }) {
  const presentation = accountErrorPresentation(error);
  return (
    <Alert variant={presentation.variant}>
      <AlertTitle>{presentation.title}</AlertTitle>
      <AlertDescription>{presentation.description}</AlertDescription>
    </Alert>
  );
}
