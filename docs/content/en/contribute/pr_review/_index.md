---
title: "Pull Request Review Guidelines"
linkTitle: "PR Review Guidelines"
no_list: false
weight: 12
description: "How to conduct a code review for a pull request in the K8ssandra project."
---

## PR review objectives

### PR review primary objectives (blocking)

#### Verify that the PR implements the changes described in the linked issue

Pull requests should be linked to issues through the presence of “Fixes #xxx” in the PR description.

Issues should have enough information to allow the contributor to implement the changes, and for the reviewer to be able to check the implementation matches the expectations.

The definition of done should be present on the issue and list the main expectations to consider the work as being ready for review. This is not an exhaustive list and should only contain the main expected changes.

The reviewer can refer to the issue description and definition of done to request blocking changes.

If the description is not accurate anymore because the requirements have changed, it should be updated to match the implementation.

#### Verify that the PR doesn’t introduce bugs
A PR shouldn’t introduce bugs or broken features. The reviewer should check the code and manually test the changes to chase down potential bugs.

Identifying a (potential) bug is a valid reason for a blocking change request.

#### Verify that new code is covered by tests
All newly introduced code should be covered by tests. Codecov can be used to check the unit test coverage on most of our tools. We do not have tooling in place to check the coverage of our e2e tests, which needs to be checked manually.

Code that isn’t tested in CI should be considered as broken.

Exception paths tests are “good to have” but not a hard requirement.

The lack of testing of newly introduced code/features is a valid reason for a blocking change request.

#### Verify that the new features are documented as part of the PR
If a PR introduces a new feature or modifies the behavior of existing features, it needs to contain updates to the in tree documentation.

Our tools are as usable as they are documented and users shouldn’t have to dig into the code or CRD definitions to discover our features.

The lack of documentation update is a valid reason for a blocking change request.

#### Verify that the code design follows defined policies
The team can settle on specific code design policies. These policies have to be documented under “docs/…/contribute” or “docs/…/developer” depending on the projects so that they can be referred to during a code review.

Examples for this are the branching model and the requirement of rebasing feature branches on top of the main branch instead of merging main into the feature branch.

Another (upcoming) example could be around our usage of labels and how we apply them to objects that are created during the reconciliation process.

Not conforming to such documented policies is a valid reason for a blocking change request.
It is not possible to ask for a blocking change on a code design issue that has no documented policy.

We primarily rely on linters/checkstyle to enforce code style conventions:
- Kubernetes operators: golang/lint
- Reaper: checkstyle
- Medusa: flake8

Adding exceptions and nolint annotations should be documented and motivated. The reviewer is entitled to ask for a blocking change if the reason behind the exception isn’t considered as a valid one.

#### Assess the injection of new dependencies
Injecting new dependencies can have long term implications for our projects. The contributor is primarily responsible for getting approval from the team when introducing new dependencies.
The reviewer needs to assess if newly introduced dependencies that could be controversial were pre-approved by the team.

In the case where the reviewer identifies that a newly injected dependency is a threat for the stability of the project, he is entitled to block the approval and must start a team wide conversation to address the issue.

### PR review secondary objectives (non blocking)

#### Code style improvements
The review process allows improving the style of the newly added code. While it is important to ensure a consistent and elegant code style, these are very subjective things.

As such, it is important to keep these comments as an open conversation that can benefit both parties, but ultimately it’s the contributor who gets to decide whether or not to apply changes. Code style improvements cannot be blocking change requests.

They can be subdivided in two categories: nitpicks and suggestions.

Nitpicks are trivial by essence and can be applied or discarded without follow ups.

Suggestions can lead to the creation of refactoring tickets if they’re not addressed in the PR. Either the contributor or the reviewer can create and address the refactoring ticket. It is highly desirable that the actual refactoring is done by one of these two involved team members as a follow up during a refactoring week, as they have all the required context to address this.

If the contributor is opposed to a refactoring of the code, it needs to be escalated so that we can reach a decision with the wider team.


#### Improve collaboration and code knowledge
The code review process isn’t a confrontation. It is a moment of collaboration and exchange. The contributor can get another view on her/his work to improve it, and get some confidence that it is implementing what it’s supposed to, without bugs.

For the reviewer, it’s an opportunity to keep track of changes happening in the code and provide some valuable feedback to the contributor.

This feedback being provided in pure textual form, comments need to be carefully written so that they’re easily understood and perceived by the other end as constructive. Refer to the comment convention in the next section.

## Comments convention
In order to minimize the odds of misunderstandings and properly convey the intent of each comment, the following convention is proposed, based on the more exhaustive [conventionalcomments.org](https://conventionalcomments.org).

### Comment labels

| **Label**       | **Description**                                                                                                                                                                                                                                                                                      | **Blocking** |
|-----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:------------:|
| **nitpick:**    | Trivial preference based request.                                                                                                                                                                                                                                                                    |      No      |
| **suggestion:** | Propose an improvement.  It can be about code design, improved performance, improved logic, etc… Suggestions should lead to refactoring tickets unless addressed by the contributor straight away or discarded by the reviewer after a conversation.                                                 |      No      |
| **issue:**      | Highlight a specific problem, such as a bug. It is requested to pair such comments with change suggestions (preferably using the built in feature that exists in GH for this if applicable). Spotting an issue isn’t enough and can leave the contributor confused if no suggestion is made with it. |      Yes     |
| **todo:**       | Necessary change or action to perform before approval.                                                                                                                                                                                                                                               |      Yes     |
| **thought:**    | Thoughts represent an idea that popped up from reviewing. These comments are non-blocking by nature, but they are extremely valuable and can lead to more focused initiatives and mentoring opportunities.                                                                                           |      No      |
| **question:**   | Ask for more details to help push the review forward. Use when a conversation is needed to define whether or not a change request is needed.                                                                                                                                                         |      No      |

Such labels are required on the first comment of a conversation, and not on the follow-up messages in that same conversation.

## Recommended practices
### Leave actionable comments

Making comments actionable in the context of code review is important for several reasons:

- **Efficiency**: Actionable comments provide clear steps on what needs to be done to improve the code. Without clear direction, the recipient of the feedback may not know how to proceed or may need to spend additional time figuring out what to do, which slows down the development process.
- **Effectiveness**: When comments are actionable, they are more likely to result in meaningful improvements. Vague or non-specific comments can lead to confusion and may not result in the desired changes.
- **Learning**: Providing specific, actionable feedback helps the code author understand not just what is wrong, but how they can fix it. This contributes to their growth and development as a programmer.
- **Constructiveness**: Feedback that includes actionable steps is generally seen as more constructive because it shows a path forward. It demonstrates that the reviewer has thought about possible solutions and is invested in helping improve the code, not just pointing out flaws.

For instance, instead of just saying "This code is hard to understand", an actionable comment would be "The function 'calculate_tax' is a bit hard to understand. We could consider breaking it down into smaller functions, each performing a distinct subtask, or adding more descriptive comments to explain the logic."

Remember, actionable feedback should ideally be specific, understandable, and feasible, providing a clear path for the recipient to follow.


### Replace “you” with “we”
Using "we" instead of "you" in pull request review comments is often recommended as a part of creating a collaborative, positive, and non-blaming culture in a development team. Here's why:

- **Promotes collaboration**: Using "we" helps foster a sense of teamwork and collective responsibility. It sends the message that everyone is working together to improve the codebase.
- **Avoids blame**: Saying "you" can sometimes come across as accusatory or blaming, especially in written form where tone can be hard to interpret. "We" can help soften the tone and ensure the feedback is taken as constructive rather than personal criticism.
- **Enhances learning**: When you use "we", it opens up the discussion for potential learning opportunities. The problem can be addressed as a shared challenge to be overcome together, rather than something that one person needs to fix on their own.
- **Cultivates a positive culture**: Using "we" contributes to a positive culture where everyone feels they're a valued part of the team. It encourages openness, respect, and inclusiveness.

**As much as possible, and when it makes sense, try to replace “you” with “we” when posting comments to soften the tone of comments.**

### Replace “should” with “could”
The recommendation to use "could" instead of "should" in pull request review comments (or in any other type of feedback) stems from similar principles of promoting a more positive and respectful communication culture. Here are a few reasons:

- **Offers suggestion rather than instruction**: Using "could" suggests a possibility or an option, rather than an instruction or an order. This respects the original author's autonomy and invites them to consider the suggestion rather than feeling compelled to implement it.
- **Softens the tone**: "Could" softens the tone of the feedback, making it sound less directive and more like a suggestion or an option for consideration. This can make it easier for the recipient to receive and consider the feedback.
- **Promotes dialogue and understanding**: By suggesting rather than instructing, "could" opens up room for discussion. The original author might have reasons for their choices that aren't apparent at first glance. Using "could" instead of "should" can invite a more open exchange of ideas and better mutual understanding.

For example, instead of saying "You should refactor this code for better readability," you could say "Could we consider refactoring this code for better readability?" This kind of language promotes a more respectful, collaborative, and open discussion around code improvements.

**It is recommended to use “could” instead of “should” to soften the tone of a change request if it can come up as too direct.**

### Avoid being sarcastic
Humor can be hard to convey in written form. This is compounded by the asynchronous nature of reviews, our different cultural backgrounds, and the fact that not everyone speaks english as a primary language.

If your jokes are not understood, they will fall flat and may offend. In particular, sarcasm will almost always come off as pedantic and condescending.

Code reviews are not the place for witty banter, we have other outlets for that (in-person meetings, Slack). The tone should be neutral and factual.

Example (from [this great post from Sandya Sankarram](https://medium.com/@sandya.sankarram/unlearning-toxic-behaviors-in-a-code-review-culture-b7c295452a3c)):
- **Unhelpful**: "Did you even test this code before you checked it in?"
- **Helpful**: "This breaks when you enter a negative number. Can you please address this case?"

### Acknowledge all comments
Whatever the nature of the comments (blocking or non-blocking), contributors have to review and acknowledge all of them, even if they are discarded.
We should be mindful of explaining in a respectful manner why we’re discarding a suggestion or a nitpick.  
This allows the reviewer to ensure that all comments were considered.
