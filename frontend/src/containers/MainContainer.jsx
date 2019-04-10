import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import * as clipboard from 'clipboard-polyfill';
import autobind from 'autobind-decorator';
import colors from 'colors.scss';

import { refreshEnvironments } from 'actions/environments';
import UserMenu from 'components/UserMenu';
import Button from '../components/Button';
import Hint from '../components/Hint';
import Icon from '../components/Icon';
import { notify } from '../components/Notification';

import 'containers/MainContainer.scss';

const POLLING_INTERVAL = 10000;

class MainContainer extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewHint: false,
      showUserMenu: false
    };

    this.props.dispatch(refreshEnvironments());
    this.poll = setInterval(this.handlePollEnvironments, POLLING_INTERVAL);
  }

  componentWillUnmount() {
    clearInterval(this.poll);
  }

  @autobind
  handlePollEnvironments() {
    this.props.dispatch(refreshEnvironments());
  }

  render() {
    const { environments, user } = this.props;
    return (
      <div className="MainContainer layout vertical">
        <div className="Header layout horizontal center">
          {`${user.username}'s space`}
          <div className="flex-auto" />
          <div className="User">
            <div className="UserAtom layout horizontal center"
              onClick={() => this.setState({ showUserMenu: true })}>
              <div className="Avatar">
                <Icon icon="logo" size="52" color={colors.navyDark} />
              </div>
              <div className="Username">{user.username}</div>
              <Icon icon="plus" size="12" color="white" />
            </div>
            {this.state.showUserMenu && 
              <UserMenu
                user={user} 
                onLogout={() => {}}
                onClose={() => this.setState({ showUserMenu: false })}
              />
            }
          </div>
        </div>

        <div className="EnvironmentList layout vertical">
          {Object.keys(environments).map(id => 
            <div key={id}className="EnvironmentItem layout horizontal start">
              <Icon className="Icon" icon="mirror" size="20"/>
              <div className="Name">{environments[id].name}</div>
              <div className="Endpoints layout vertical">
                {environments[id].endpoints.map(url =>
                  <a key={`${id}-${url}`}>{url}</a>
                )}
              </div>
            </div>
          )}
        </div>

        <div className="ActionBar layout horizontal center">
          <div className="flex-auto"></div>
          <div className="NewButtonContainer">
            <Button 
              className="NewButton" 
              icon="plusCircle" 
              iconSize="18"
              onClick={() => this.setState({ showNewHint: true })}
            >
              New Environment
            </Button>
            <Hint 
              className="HintNew"
              open={this.state.showNewHint}
              onClose={() => this.setState({ showNewHint: false })}
              width="420"
              arrowPosition="right"
              offsetY="46"
              offsetX="-50"
              positionX="left"
            >
              <div className="HintNewContainer layout vertical">
                <h1>Launch Environment</h1>
                <p>
                  Launch <strong>Okteto CLI</strong>
                  &nbsp;from your <strong>local repository</strong>:
                </p>
                <div className="layout horizontal">
                  <code className="cli flex-auto">
                    okteto up
                  </code>
                  <Button
                    icon="copy"
                    onClick={() => {
                      clipboard.writeText(`okteto up`);
                      notify('Copied to clipboard!');
                      // mixpanel.track('Copied CLI Command');
                      this.setState({ showNewHint: false });
                    }}
                    light>
                  </Button>
                </div>
              </div>
            </Hint>
          </div>
        </div>
      </div>
    );  
  }
}

MainContainer.defaultProps = {
};

MainContainer.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired,
  environments: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {},
    user: state.session.user
  };
})(MainContainer);