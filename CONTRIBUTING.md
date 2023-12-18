# Contributing Guidelines

The Rollouts-Plugin-TrafficRouter-Contour project accepts contributions via GitHub pull requests. This document outlines the process
to help get your contribution accepted.

## Reporting a Security Issue

Most of the time, when you find a bug in Rollouts-Plugin-TrafficRouter-Contour, it should be reported using [GitHub
issues](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/issues). This will give us a
chance to try to fix the issue before it is exploited in the wild.

## Sign Your Work

The sign-off is a simple line at the end of the explanation for a commit. All commits need to be
signed. Your signature certifies that you wrote the patch or otherwise have the right to contribute
the material. The rules are pretty simple, if you can certify the below (from
[developercertificate.org](https://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@example.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your commit automatically
with `git commit -s`.

Note: If your git config information is set properly then viewing the `git log` information for your
 commit will look something like this:

```
Author: Joe Smith <joe.smith@example.com>
Date:   Thu Feb 2 11:41:15 2018 -0800

    Update README

    Signed-off-by: Joe Smith <joe.smith@example.com>
```

Notice the `Author` and `Signed-off-by` lines match. If they don't your PR will be rejected by the
automated DCO check.

## Support Channels

Whether you are a user or contributor, official support channels include:

- [Issues](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/issues)

Before opening a new issue or submitting a new pull request, it's helpful to search the project -
it's likely that another user has already reported the issue you're facing, or it's a known issue
that we're already aware of.

## Semantic Versioning

We maintain a strong commitment to backward compatibility. All of our changes to protocols and
formats are backward compatible from one major release to the next. No features, flags, or commands
are removed or substantially modified (unless we need to fix a security issue).

We also try very hard to not change publicly accessible Go library definitions inside of the `pkg/`
directory of our source code.

- Command line commands, flags, and arguments MUST be backward compatible
- File formats (such as Chart.yaml) MUST be backward compatible
- Chart repository functionality MUST be backward compatible
- Go libraries inside of `pkg/` SHOULD remain backward compatible.

## Issues

Issues are used as the primary method for tracking anything to do with the Rollouts-Plugin-TrafficRouter-Contour project.

### Issue Types

There are 5 types of issues (each with their own corresponding [label](#labels)):

- `question/support`: These are support or functionality inquiries that we want to have a record of
  for future reference. Generally these are questions that are too complex or large to store in the
  Slack channel or have particular interest to the community as a whole. Depending on the
  discussion, these can turn into `feature` or `bug` issues.
- `proposal`: Used for items (like this one) that propose a new ideas or functionality that require
  a larger community discussion. This allows for feedback from others in the community before a
  feature is actually  developed. This is not needed for small additions. Final word on whether or
  not a feature needs a proposal is up to the core maintainers. All issues that are proposals should
  both have a label and an issue title of "Proposal: [the rest of the title]." A proposal can become
  a `feature` and does not require a milestone.
- `feature`: These track specific feature requests and ideas until they are complete. They can
  evolve from a `proposal` or can be submitted individually depending on the size.
- `bug`: These track bugs with the code
- `docs`: These track problems with the documentation (i.e. missing or incomplete)

### Issue Lifecycle

The issue lifecycle is mainly driven by the core maintainers, but is good information for those
contributing to Rollouts-Plugin-TrafficRouter-Contour. All issue types follow the same general lifecycle. Differences are noted
below.

1. Issue creation
2. Triage
    - The maintainer in charge of triaging will apply the proper labels for the issue. This includes
      labels for priority, type, and metadata (such as `good first issue`). The only issue priority
      we will be tracking is whether or not the issue is "critical." If additional levels are needed
      in the future, we will add them.
    - (If needed) Clean up the title to succinctly and clearly state the issue. Also ensure that
      proposals are prefaced with "Proposal: [the rest of the title]".
    - Add the issue to the correct milestone. If any questions come up, don't worry about adding the
      issue to a milestone until the questions are answered.
    - We attempt to do this process at least once per work day.
3. Discussion
    - Issues that are labeled `feature` or `proposal` must write a Rollouts-Plugin-TrafficRouter-Contour Improvement Proposal (HIP).
      See [Proposing an Idea](#proposing-an-idea). Smaller quality-of-life enhancements are exempt.
    - Issues that are labeled as `feature` or `bug` should be connected to the PR that resolves it.
    - Whoever is working on a `feature` or `bug` issue (whether a maintainer or someone from the
      community), should either assign the issue to themselves or make a comment in the issue saying
      that they are taking it.
    - `proposal` and `support/question` issues should stay open until resolved or if they have not
      been active for more than 30 days. This will help keep the issue queue to a manageable size
      and reduce noise. Should the issue need to stay open, the `keep open` label can be added.
4. Issue closure

## How to Contribute a Patch

1. Identify or create the related issue. If you're proposing a larger change to
   Rollouts-Plugin-TrafficRouter-Contour, see [Proposing an Idea](#proposing-an-idea).
2. Fork the desired repo; develop and test your code changes.
3. Submit a pull request, making sure to sign your work and link the related issue.

## Pull Requests

Like any good open source project, we use Pull Requests (PRs) to track code changes.

### PR Lifecycle

1. PR creation
    - PRs are usually created to fix or else be a subset of other PRs that fix a particular issue.
    - We more than welcome PRs that are currently in progress. They are a great way to keep track of
      important work that is in-flight, but useful for others to see. If a PR is a work in progress,
      it **must** be prefaced with "WIP: [title]". Once the PR is ready for review, remove "WIP"
      from the title.
    - It is preferred, but not required, to have a PR tied to a specific issue. There can be
      circumstances where if it is a quick fix then an issue might be overkill. The details provided
      in the PR description would suffice in this case.
2. Triage
    - The maintainer in charge of triaging will apply the proper labels for the issue. This should
      include at least a size label, `bug` or `feature`, and `awaiting review` once all labels are
      applied. See the [Labels section](#labels) for full details on the definitions of labels.
    - Add the PR to the correct milestone. This should be the same as the issue the PR closes.
3. Assigning reviews
    - Once a review has the `awaiting review` label, maintainers will review them as schedule
      permits. The maintainer who takes the issue should self-request a review.
    - PRs from a community member with the label `size/S` or larger requires 2 review approvals from
      maintainers before it can be merged. Those with `size/XS` are per the judgement of the
      maintainers. For more detail see the [Size Labels](#size-labels) section.
4. Reviewing/Discussion
    - All reviews will be completed using GitHub review tool.
    - A "Comment" review should be used when there are questions about the code that should be
      answered, but that don't involve code changes. This type of review does not count as approval.
    - A "Changes Requested" review indicates that changes to the code need to be made before they
      will be merged.
    - Reviewers should update labels as needed (such as `needs rebase`)
5. Address comments by answering questions or changing code
6. LGTM (Looks good to me)
    - Once a Reviewer has completed a review and the code looks ready to merge, an "Approve" review
      is used to signal to the contributor and to other maintainers that you have reviewed the code
      and feel that it is ready to be merged.
7. Merge or close
    - PRs should stay open until merged or if they have not been active for more than 30 days. This
      will help keep the PR queue to a manageable size and reduce noise. Should the PR need to stay
      open (like in the case of a WIP), the `keep open` label can be added.
    - Before merging a PR, refer to the topic on [Size Labels](#size-labels) below to determine if
      the PR requires more than one LGTM to merge.
    - If the owner of the PR is listed in the `OWNERS` file, that user **must** merge their own PRs
      or explicitly request another OWNER do that for them.
    - If the owner of a PR is _not_ listed in `OWNERS`, any core maintainer may merge the PR.

#### Documentation PRs

Documentation PRs will follow the same lifecycle as other PRs. They will also be labeled with the
`docs` label. For documentation, special attention will be paid to spelling, grammar, and clarity
(whereas those things don't matter *as* much for comments in code).

