import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import * as clipboard from 'clipboard-polyfill';

import Button from '../components/Button';
import Hint from '../components/Hint';
import Icon from '../components/Icon';
import { notify } from '../components/Notification';

import 'containers/MainContainer.scss';
import colors from 'colors.scss';

class MainContainer extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewHint: false
    };
  }

  render() {
    const { environments } = this.props;
    return (
      <div className="MainContainer layout vertical">
        <div className="Header layout horizontal center">
          space
        </div>
        <div className="EnvironmentList layout vertical">
          {Object.keys(environments).map(id => 
            <div key={id}className="EnvironmentItem layout horizontal center">
              <Icon className="Icon" icon="mirror" size="20"/>
              <div className="Name">{environments[id].name}</div>
              <div className="Endpoint">
                <a>{environments[id].endpoint}</a>
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
  environments: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {}
  };
})(MainContainer);