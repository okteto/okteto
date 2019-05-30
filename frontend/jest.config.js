module.exports = {
  moduleFileExtensions: ['js', 'jsx'],
  moduleDirectories: ['node_modules'],
  moduleNameMapper: {
    '\\.(css|scss)$': 'identity-obj-proxy'
  },
  modulePaths: ['<rootDir>/src/'],
  globals: {
    'VERSION': 'test-' + Date.now(),
    'MODE': 'production',
    'GITHUB_CLIENT_ID': 'none'
  },
  setupFiles: ['jest-localstorage-mock']
};