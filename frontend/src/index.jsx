import React from 'react';
import ReactDOM from 'react-dom';
import { createStore, applyMiddleware, compose } from 'redux';
import { Provider } from 'react-redux';
import thunk from 'redux-thunk';
import smoothscroll from 'smoothscroll-polyfill';
import 'whatwg-fetch';

import 'index.scss';

import reducers from 'reducers';
import environment from 'common/environment';
import AppView from 'views/AppView';

let store;
if (environment.mode === 'development') {
  // To enable Redux devtools.
  const composeEnhancers = window.__REDUX_DEVTOOLS_EXTENSION_COMPOSE__ || compose;
  store = createStore(reducers, composeEnhancers(applyMiddleware(thunk)));
} else {
  store = createStore(reducers, applyMiddleware(thunk));
}

smoothscroll.polyfill();

ReactDOM.render(
  <Provider store={ store }>
    <AppView />
  </Provider>,
  document.getElementById('app')
);

