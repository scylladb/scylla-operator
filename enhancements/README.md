# Enhancements Tracking and Backlog

Inspired by the [Kubernetes enhancement](https://github.com/kubernetes/enhancements) process.

This folder holds enhancements for scylla-operator. Enhancements are a way of discussing, debating and reaching consensus on how a change should be delivered as well as documenting it. 

Enhancements may be filed from anyone in the community, but require consensus from the maintainers, and a person willing to commit cycles to the implementation.

## Is My Thing an Enhancement?

A rough heuristic for an enhancement is anything that:

- impacts how a scylla is operated including addition or removal of significant
  capabilities
- impacts upgrade/downgrade
- needs significant effort to complete
- requires broader consensus
- proposes adding a new user-facing component
- demands formal documentation to utilize

It is unlikely to require an enhancement if it:

- fixes a bug
- adds more testing
- internally refactors a code or component only visible to that components
  domain
- minimal impact to distribution as a whole

If you are not sure if the proposed work requires an enhancement, file an issue
and ask!
