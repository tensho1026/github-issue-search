import { useParams } from "react-router";

import { ProfileDashboard } from "../features/profile/components/ProfileDashboard";
import { ProfileErrorState } from "../features/profile/components/ProfileErrorState";
import { ProfileLoadingState } from "../features/profile/components/ProfileLoadingState";
import { useProfileSnapshot } from "../features/profile/api/useProfileSnapshot";
import { ApiError } from "../shared/api/client";
import { validateGitHubUsername } from "../shared/lib/github-username";

export function ProfilePage() {
  const { username: routeUsername = "" } = useParams<{
    username: string;
  }>();
  const validation = validateGitHubUsername(routeUsername);
  const username = validation.valid ? validation.username : routeUsername;
  const { error, isFetching, isPending, refetch, snapshot } =
    useProfileSnapshot(username, validation.valid);

  if (!validation.valid) {
    return (
      <ProfileErrorState
        error={
          new ApiError({
            code: "INVALID_REQUEST",
            message: validation.message,
            status: 400,
          })
        }
        isFetching={false}
        onRetry={() => undefined}
        username={username}
      />
    );
  }
  if (isPending) {
    return <ProfileLoadingState />;
  }
  if (error) {
    return (
      <ProfileErrorState
        error={error}
        isFetching={isFetching}
        onRetry={() => {
          void refetch();
        }}
        username={username}
      />
    );
  }
  if (!snapshot) {
    return <ProfileLoadingState />;
  }
  return <ProfileDashboard snapshot={snapshot} />;
}
