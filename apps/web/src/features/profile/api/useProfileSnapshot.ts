import { useQuery } from "@tanstack/react-query";
import { useCallback } from "react";

import { getGitHubUser, getProfileAnalysis } from "../../../shared/api/profile";
import type {
  GitHubUser,
  Meta,
  ProfileAnalysis,
} from "../../../shared/api/generated";
import { queryKeys } from "../../../shared/query/query-keys";
import { prioritizedProfileError } from "../model/profile-error";

export type ProfileSnapshot = {
  analysis: ProfileAnalysis;
  analysisMeta: Meta;
  user: GitHubUser;
  userMeta: Meta;
};

export function useProfileSnapshot(username: string, enabled = true) {
  const userQuery = useQuery({
    enabled,
    queryFn: ({ signal }) => getGitHubUser(username, signal),
    queryKey: queryKeys.profile.user(username),
  });
  const analysisQuery = useQuery({
    enabled,
    queryFn: ({ signal }) => getProfileAnalysis(username, signal),
    queryKey: queryKeys.profile.analysis(username),
  });

  const refetch = useCallback(async () => {
    await Promise.all([userQuery.refetch(), analysisQuery.refetch()]);
  }, [analysisQuery, userQuery]);

  const snapshot =
    userQuery.data && analysisQuery.data
      ? {
          analysis: analysisQuery.data.data,
          analysisMeta: analysisQuery.data.meta,
          user: userQuery.data.data,
          userMeta: userQuery.data.meta,
        }
      : undefined;

  return {
    error: prioritizedProfileError([userQuery.error, analysisQuery.error]),
    isFetching: userQuery.isFetching || analysisQuery.isFetching,
    isPending: userQuery.isPending || analysisQuery.isPending,
    refetch,
    snapshot,
  };
}
