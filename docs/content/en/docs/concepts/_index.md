---
title: "Core Concepts"
linkTitle: "Concepts"
description: "Understand the foundational concepts of FHIRPath as implemented in the FHIRPath Go library: the type system, collections, operators, and environment variables."
weight: 2
---

FHIRPath is a path-based navigation and extraction language designed for use with FHIR® resources. Before diving into advanced usage, it is important to understand the core concepts that govern how expressions are evaluated.

This section covers:

- **[Type System]({{< relref "type-system" >}})** -- The eight primitive types (Boolean, Integer, Decimal, String, Date, DateTime, Time, Quantity), their Go representations, and the `Value`, `Comparable`, and `Numeric` interfaces.

- **[Collections]({{< relref "collections" >}})** -- How FHIRPath represents all results as ordered collections, the rules for empty propagation (three-valued logic), singleton evaluation, and the full set of collection operations.

- **[Operators]({{< relref "operators" >}})** -- Arithmetic, comparison, equality, equivalence, Boolean, collection, and type operators, including precedence rules and three-valued truth tables.

- **[Environment Variables]({{< relref "environment-variables" >}})** -- Built-in variables (`%resource`, `%context`, `%ucum`) and how to define custom variables with `WithVariable()`.
