import { appRoutes } from "../config/app-config";

export const maximumReturnLocationLength = 4096;

export function issueDetailSearchParameters(
  currentLocation: string,
): URLSearchParams {
  const parameters = new URLSearchParams();
  if (
    currentLocation.length <= maximumReturnLocationLength &&
    currentLocation.startsWith(`${appRoutes.search}?`)
  ) {
    parameters.set("from", currentLocation);
  }
  return parameters;
}
