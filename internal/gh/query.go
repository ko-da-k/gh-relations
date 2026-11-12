// Package gh is the only part of gh-relations that talks to GitHub. It sends
// one GraphQL document, turns the response into graph.Observation values, and
// hands those to the walk.
package gh

// pageSizeLiteral is how many entries of one link connection a request asks
// for. A longer connection keeps its first page and the item is marked as
// having links that were not seen, since paginating would spend requests inside
// a level, which --depth cannot see.
const pageSizeLiteral = "100"

// defaultIDsPerRequest is the assumed cap on nodes(ids:). A level with more ids
// than this is split, which is the one way a level costs more than one request.
const defaultIDsPerRequest = 100

// refFragment is what is selected for an item only named by someone else's
// response: enough to draw it, and the id needed to fetch it later. The two
// inline fragments cannot be merged, because a pull request has no parent,
// subIssues or blockedBy and naming those under PullRequest is a schema error.
const refFragment = `
fragment Ref on Node {
  __typename
  ... on Issue {
    id number title url state stateReason
    repository { nameWithOwner }
  }
  ... on PullRequest {
    id number title url state isDraft
    repository { nameWithOwner }
  }
}`

// timelineSelection is asked for once with both event types and split apart by
// __typename. Both values exist in the Issue and the PullRequest enum.
//
// Truncation is decided from hasNextPage rather than totalCount, because
// PullRequest.timelineItems reports a totalCount that ignores itemTypes.
const timelineSelection = `
    timelineItems(first: ` + pageSizeLiteral + `, itemTypes: [CROSS_REFERENCED_EVENT, MARKED_AS_DUPLICATE_EVENT]) {
      pageInfo { hasNextPage }
      nodes {
        __typename
        ... on CrossReferencedEvent { source { ...Ref } }
        ... on MarkedAsDuplicateEvent { canonical { ...Ref } duplicate { ...Ref } }
      }
    }`

// linksFragment is every structured link field the extension reads. There is no
// reverse-lookup API for "which timelines name this item", so these fields are
// the whole of what can be known without parsing prose.
const linksFragment = `
fragment Links on Node {
  ...Ref
  ... on Issue {
    parent { ...Ref }
    subIssues(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    blockedBy(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    blocking(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    relatesTo(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    duplicateOf { ...Ref }
    trackedIssues(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    trackedInIssues(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }
    closedByPullRequestsReferences(first: ` + pageSizeLiteral + `, includeClosedPrs: true) {
      pageInfo { hasNextPage } nodes { ...Ref }
    }` + timelineSelection + `
  }
  ... on PullRequest {
    closingIssuesReferences(first: ` + pageSizeLiteral + `) { pageInfo { hasNextPage } nodes { ...Ref } }` +
	timelineSelection + `
  }
}`

// rootQuery resolves the starting item. issueOrPullRequest takes a number
// without being told which of the two kinds it is.
const rootQuery = refFragment + linksFragment + `
query RelationsRoot($owner: String!, $name: String!, $number: Int!) {
  rateLimit { cost nodeCount remaining }
  repository(owner: $owner, name: $name) {
    issueOrPullRequest(number: $number) { ...Links }
  }
}`

// levelQuery fetches a whole level. The cost of a request is 1 whatever the id
// count, so batching a level is what keeps the rate limit flat.
const levelQuery = refFragment + linksFragment + `
query RelationsLevel($ids: [ID!]!) {
  rateLimit { cost nodeCount remaining }
  nodes(ids: $ids) { ...Links }
}`
