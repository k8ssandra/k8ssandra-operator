# K8ssandra.io - Website & Documentation Repository

This project is built with the [Hugo](https://gohugo.io/) static site generator and the [Docsy](https://github.com/google/docsy) theme.

The documentation produced from this repository can be viewed at:

Development Version --  https://docs-staging.k8ssandra.io

Production Version -- https://docs.k8ssandra.io

## Dependencies

This project requires Node.js and NPM to be installed locally.  All other dependencies are provided via NPM.

The latest version of Node/NPM should work, the current automation uses Node 14.

You can install Node in a myriad of ways, check [here](https://nodejs.org/en/) for more information.

Hugo must be installed on the local machine to build the website. Please follow the installation steps from [the Hugo documentation](https://gohugo.io/installation/)
Tip: Install hugo extended.

Docsy is added as hugo module. No external installations are required for docsy.

## Development
### Install dependencies

From the `/docs` directory, run

```
npm install
```

This command will install the project dependencies, such as hugo-extended.

### Scripts

There are a number of utility scripts provided in package.json that can be executed for various purposes:

#### Start the Hugo server for local development

```
npm run start
```

This provides a live-refresh server that will automatically load any changes made in the source code.  

The local server will be available at http://localhost:1313/.

#### Use Hugo to build the site
for local use

```
npm run build:staging
```

or
for production

```
npm run build:production
```

#### Cleanup the build artifacts previously produced

```
npm run clean
```

#### Clearing cache in case of error
Sometimes you will see errors while building sites. For example: ``template for shortcode "alert" not found``

This can be easily fixed by removing cache using.

```
hugo mod clean
```


