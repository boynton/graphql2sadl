# graphql2sadl
A simple tool to convert GraphQL logical data models to [SADL](https://github.com/boynton/sadl).

## Usage

Assuming you have Go installed on your machine:

    $ go get github.com/boynton/graphql2sadl
    $ graphql2sadl yourfile.graphql

This outputs the SADL types represented by the graphql source, in SADL soruce form. To output the SADL JSON representation:

    $ graphql2sadl -j yourfile.graphql


_Note: this tool is incomplete, more of a demo than anything._
