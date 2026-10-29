/* eslint-env node */

import * as path from 'path';
import { Configuration as WebpackConfiguration } from 'webpack';
import { Configuration as WebpackDevServerConfiguration } from 'webpack-dev-server';
import { ConsoleRemotePlugin, EncodedExtension } from '@openshift-console/dynamic-plugin-sdk-webpack';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CopyWebpackPlugin = require('copy-webpack-plugin');

const isProd = process.env.NODE_ENV === 'production';

// Node status titles are static, so the Nodes list shows the drain progress by one "Draining <n>%" status
// per progress value; see isDrainingAt in src/nodeStatus.tsx, which must use the same resource key prefix.
const drainProgressResourcePrefix = 'drainProgress';
const drainProgressExtensions: EncodedExtension[] = Array.from({ length: 101 }, (_, progress) => ({
  type: 'console.node/status',
  properties: {
    title: `Draining ${progress}%`,
    resources: {
      [`${drainProgressResourcePrefix}${progress}`]: {
        groupVersionKind: {
          group: 'nodemaintenance.medik8s.io',
          version: 'v1beta1',
          kind: 'NodeMaintenance',
        },
        isList: true,
      },
    },
    isActive: { $codeRef: 'nodeStatus.isDrainingAt' },
    PopoverContent: { $codeRef: 'nodeStatus.DrainProgressPopover' },
  },
}));

// eslint-disable-next-line @typescript-eslint/no-var-requires
const staticExtensions: EncodedExtension[] = require('./console-extensions.json');

interface Configuration extends WebpackConfiguration {
  devServer?: WebpackDevServerConfiguration;
}

const config: Configuration = {
  mode: isProd ? 'production' : 'development',
  // No regular entry points needed. All plugin related scripts are generated via ConsoleRemotePlugin.
  entry: {},
  context: path.resolve(__dirname, 'src'),
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: isProd ? '[name]-bundle-[hash].min.js' : '[name]-bundle.js',
    chunkFilename: isProd ? '[name]-chunk-[chunkhash].min.js' : '[name]-chunk.js',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js', '.jsx'],
  },
  module: {
    rules: [
      {
        test: /\.(jsx?|tsx?)$/,
        exclude: /\/node_modules\//,
        use: [
          {
            loader: 'ts-loader',
            options: {
              configFile: path.resolve(__dirname, 'tsconfig.json'),
            },
          },
        ],
      },
      {
        test: /\.(m?js)$/,
        resolve: {
          fullySpecified: false,
        },
      },
    ],
  },
  devServer: {
    static: './dist',
    port: 9001,
    // Allow Bridge running in a container to connect to the plugin dev server.
    allowedHosts: 'all',
    headers: {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, PATCH, OPTIONS',
      'Access-Control-Allow-Headers': 'X-Requested-With, Content-Type, Authorization',
    },
    devMiddleware: {
      writeToDisk: true,
    },
  },
  plugins: [
    new ConsoleRemotePlugin({ extensions: [...staticExtensions, ...drainProgressExtensions] }),
    new CopyWebpackPlugin({
      patterns: [{ from: path.resolve(__dirname, 'locales'), to: 'locales' }],
    }),
  ],
  devtool: isProd ? false : 'source-map',
  optimization: {
    chunkIds: isProd ? 'deterministic' : 'named',
    minimize: isProd,
  },
};

export default config;
