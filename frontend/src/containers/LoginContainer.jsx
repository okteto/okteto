import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import { GoogleLogin } from 'react-google-login';
import autobind from 'autobind-decorator';
import { CSSTransition } from 'react-transition-group';

import Icon from 'components/Icon';
import { notify } from 'components/Notification';
import { authenticateWithGoogle } from 'actions/session';

import 'containers/LoginContainer.scss';
import colors from 'colors.scss';

class LoginContainer extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  onLoginSuccess(response) {
    if (response.tokenId) {
      this.props.dispatch(authenticateWithGoogle(response.tokenId));
    } else {
      notify(`Login Failed: Wrong or missing Google TokenID`, 'error');
    }
  }

  @autobind
  onLoginFailure(response) {
    notify(`Login Failed: ${response.error}`, 'error');
  }

  render() {
    return (
      <CSSTransition
        in={true}
        classNames="fade"
        appear={true}
        timeout={5000}>
        <div className="LoginContainer">
          <div className="layout vertical center-center">
            <div className="logo">
              <Icon icon="okteto" size="64" />
            </div>
            <GoogleLogin
              className="Button login-button layout horizontal center"
              clientId={this.props.config.google}
              buttonText="Login with Google"
              onSuccess={this.onLoginSuccess}
              onFailure={this.onLoginFailure}>
              <Icon className="google-icon" icon="google" color={colors.white900} />
              Login with Google
            </GoogleLogin>
            <div className="terms">
              By proceeding, you agree to the 
              the <a href="https://okteto.com/legal">Terms of Service</a> and<br /> acknowledge you 
              have read the <a href="https://okteto.com/privacy-policy">Privacy Policy</a>.
            </div>
          </div>
        </div>
      </CSSTransition>
    );
  }
}

LoginContainer.propTypes = {
  session: PropTypes.object.isRequired,
  config: PropTypes.object.isRequired,
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(state => {
  return {
    config: state.config,
    session: state.session
  };
})(LoginContainer);