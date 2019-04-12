import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import * as clipboard from 'clipboard-polyfill';
import autobind from 'autobind-decorator';

import { refreshEnvironments } from 'actions/environments';
import Header from './Header';
import Button from '../components/Button';
import Hint from '../components/Hint';
import Icon from '../components/Icon';
import { notify } from '../components/Notification';

import 'containers/Space.scss';

const POLLING_INTERVAL = 10000;

class Space extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewHint: false
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
      <div className="Space layout vertical">
        <Header title={`${user.id}'s space`} />

        <div className="EnvironmentList layout vertical">
          {Object.keys(environments).map(id => 
            <div key={id} className="EnvironmentItem layout horizontal start">
              <div className="layout horizontal start">
                <Icon className="EnvironmentItemIcon" icon="mirror" size="20"/>
                <div className="EnvironmentItemName ellipsis" 
                  title={environments[id].name}>
                  {environments[id].name}
                </div>
              </div>
              <div className="EnvironmentItemEndpoints layout vertical flex-auto">
                {environments[id].endpoints.map(url =>
                  <a className="ellipsis" 
                    key={`${id}-${url}`} 
                    href={url}
                    rel="noreferrer noopener" 
                    target="_blank">
                    {url}
                  </a>
                )}
              </div>
              {/* <div className="Buttons">
                <Button icon="plus" iconSize="20" frameless />
              </div> */}
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
                    icon="clipboard"
                    iconSize="24"
                    onClick={() => {
                      clipboard.writeText(`okteto up`);
                      notify('Copied to clipboard!');
                      // mixpanel.track('Copied CLI Command');
                      this.setState({ showNewHint: false });
                    }}
                    light
                    frameless>
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

Space.defaultProps = {
};

Space.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired,
  environments: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {},
    user: state.session.user
  };
})(Space);