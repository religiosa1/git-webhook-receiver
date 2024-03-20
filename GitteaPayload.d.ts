type TimestampString = string; //  "2024-02-29T01:26:47Z" || "0001-01-01T00:00:00Z"

interface GitteaCommitInfo {
  id: string; // "157c294031ab9af3f1f6d5a23383dc9ef90509bf"
  message: string; // "MultifFileINput props.accept\n";
  url: string; // "https://git.religiosa.ru/religiosa/staticus/commit/157c294031ab9af3f1f                                                                                                             6d5a23383dc9ef90509bf";
  author: {
    name: string;
    email: string;
    username: string;
  };
  committer: {
    name: string;
    email: string;
    username: string;
  };
  verification: null;
  timestamp: TimestampString;
  added: null;
  removed: null;
  modified: null;
}

interface GitteaUserInfo {
  id: number;
  login: string;
  login_name: string;
  full_name: string;
  email: string;
  avatar_url: string;
  language: string;
  is_admin: boolean;
  last_login: TimestampString;
  created: TimestampString;
  restricted: boolean;
  active: boolean;
  prohibit_login: boolean;
  location: string;
  website: string;
  description: string;
  visibility: string; // "public";
  followers_count: number;
  following_count: number;
  starred_repos_count: number;
  username: string;
}

interface GitteaPayload {
  ref: string; // "refs/heads/master";
  before: string; // "157c294031ab9af3f1f6d5a23383dc9ef90509bf";
  after: string; // "157c294031ab9af3f1f6d5a23383dc9ef90509bf";
  compare_url: string; // "https://git.religiosa.ru/religiosa/staticus/compare/157c294031ab                                                                                                             9af3f1f6d5a23383dc9ef90509bf...157c294031ab9af3f1f6d5a23383dc9ef90509bf";
  commits: GitteaCommitInfo[];
  total_commits: number;
  head_commit: GitteaCommitInfo;
  repository: {
    id: number;
    owner: GitteaUserInfo;
    name: string; // "project";
    full_name: string; // "user/project";
    description: string;
    empty: boolean;
    private: boolean;
    fork: boolean;
    template: boolean;
    parent: null; // TODO
    mirror: boolean;
    size: number;
    language: string;
    languages_url: string; // "https://git.religiosa.ru/api/v1/repos/religiosa/staticus/lan                                                                                                             guages";
    html_url: string; // "https://git.religiosa.ru/religiosa/staticus";
    url: string; // "https://git.religiosa.ru/api/v1/repos/religiosa/staticus";
    link: string;
    ssh_url: string;
    clone_url: string;
    original_url: string;
    website: string;
    stars_count: number;
    forks_count: number;
    watchers_count: number;
    open_issues_count: number;
    open_pr_counter: number;
    release_counter: number;
    default_branch: string;
    archived: boolean;
    created_at: TimestampString;
    updated_at: TimestampString;
    archived_at: TimestampString;
    permissions: { admin: boolean; push: boolean; pull: boolean };
    has_issues: boolean;
    internal_tracker: {
      enable_time_tracker: boolean;
      allow_only_contributors_to_track_time: boolean;
      enable_issue_dependencies: boolean;
    };
    has_wiki: boolean;
    has_pull_requests: boolean;
    has_projects: boolean;
    has_releases: boolean;
    has_packages: boolean;
    has_actions: boolean;
    ignore_whitespace_conflicts: boolean;
    allow_merge_commits: boolean;
    allow_rebase: boolean;
    allow_rebase_explicit: boolean;
    allow_squash_merge: boolean;
    allow_rebase_update: boolean;
    default_delete_branch_after_merge: boolean;
    default_merge_style: string; // "merge";
    default_allow_maintainer_edit: boolean;
    avatar_url: string;
    internal: boolean;
    mirror_interval: string;
    mirror_updated: TimestampString;
    repo_transfer: null; // TODO
  };
  pusher: GitteaUserInfo;
  sender: GitteaUserInfo;
}
