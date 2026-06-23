# Security

openYuanrong components need to communicate within the cluster to complete request forwarding, task scheduling, and other functions. Component communication can use TLS encryption to ensure security.

Applications developed using openYuanrong connect to the openYuanrong cluster through clients to run. openYuanrong allows clients to run arbitrary code and has full access permissions to the cluster and underlying computing resources.

## Core Principles

In the openYuanrong ecosystem, there are three main roles: developers build applications through the openYuanrong API and test and deploy applications in local or cluster environments provided by infrastructure providers; infrastructure service providers provide computing environments for running openYuanrong for developers; users use various applications developed based on the openYuanrong platform.

To ensure the safe operation of the openYuanrong cluster, all participants need to jointly follow the following key requirements:

- Network boundary protection: All cluster deployments must implement effective network isolation strategies to ensure that external security protection measures of the cluster are in place.

- Component communication security: Infrastructure service providers need to establish a controlled isolated network environment to ensure the communication security of openYuanrong components. When the cluster needs to access third-party services, strict authentication mechanisms and access control policies must be implemented.

- Code trust management: The openYuanrong platform itself does not perform code security reviews. Developers need to conduct complete security verification and risk assessment of the deployed application code.

- Tenant and workload isolation: When there are multi-tenant or workload isolation requirements, infrastructure service providers should deploy independent and mutually isolated openYuanrong cluster environments according to the specific requirements of developers.

In short, all operations must be performed in a security-certified network environment to ensure that the platform only processes verified trusted code. Infrastructure service providers and developers should establish a joint security mechanism to build a complete security protection system through multi-layer protection such as network isolation, access control, and code auditing.

## Vulnerability Management

openYuanrong is open source in the openEuler community. If you discover security issues or suspected security vulnerabilities in openYuanrong, please follow the [Vulnerability Management](https://www.openeuler.org/en/security/vulnerability-reporting/){target="_blank"} handling mechanism provided by the openEuler community to report to the Security Committee. We will respond, analyze, and resolve quickly.

## Important Notes

1. Ensure that personnel accessing the openYuanrong cluster are trusted by restricting permissions and other measures.

2. openYuanrong uses cloudpickle to serialize Python objects. Please refer to the [pickle documentation](https://docs.python.org/3/library/pickle.html){target="_blank"} for more security model information.

3. openYuanrong components have provided core verification of certificate format, validity period, trust chain, and digital signature during runtime. If you have higher security requirements, it is recommended to expand based on the existing verification framework and refer to industry security practices (such as OpenSSL).

4. The current version includes an early implementation of multi-tenant features. This feature is still under development and has not reached production standards. The API and architecture may undergo significant adjustments in subsequent versions. Please follow our Roadmap for the official release plan.
