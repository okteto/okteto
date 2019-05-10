import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import { hot } from 'react-hot-loader';

import Notification from 'components/Notification';
import { restoreSession, logout } from 'actions/session';
import SpacesView from './SpacesView';
import LoginView from './LoginView';

import 'views/AppView.scss';

class AppView extends Component {
  constructor(props) {
    super(props);

    document.addEventListener('logout', () => {
      this.props.dispatch(logout());
    });

    // Restore any existing session.
    this.props.dispatch(restoreSession());
  }

  render() {
    return (
      <div className="AppView">
        <Notification />
        {!this.props.session.isAuthenticated &&
          <LoginView />
        }
        {this.props.session.isAuthenticated &&
          <SpacesView />
        }
      </div>
    );
  }
}

AppView.propTypes = {
  session: PropTypes.object.isRequired,
  dispatch: PropTypes.func
};

const AppViewWithRedux = ReactRedux.connect(state => {
  return {
    session: state.session
  };
})(AppView);

// Enable React Hot Loader for the root component.
export default hot(module)(AppViewWithRedux);