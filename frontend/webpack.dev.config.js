const webpack = require('webpack');
const merge = require('webpack-merge');
const baseConfig = require('./webpack.config.js');
const BundleAnalyzerPlugin = require('webpack-bundle-analyzer').BundleAnalyzerPlugin;

module.exports = merge({
  mode: 'development',
  devtool: 'source-map',
  plugins: [
    new webpack.DefinePlugin({
      MODE: JSON.stringify('development')
    }),
    new webpack.HotModuleReplacementPlugin(),
    // Uncomment this line to analyze bundle:
    // new BundleAnalyzerPlugin()
  ]
}, baseConfig);
