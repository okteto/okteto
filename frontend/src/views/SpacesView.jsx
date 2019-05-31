import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import Resizable from 're-resizable';

import Space from 'containers/Space';
import SpaceExplorer from 'containers/SpaceExplorer';
import { refreshSpaces, refreshCurrentSpace } from 'actions/spaces';

import constants from 'constants.scss';
import 'views/SpacesView.scss';

const POLLING_INTERVAL = 10000;

class SpacesView extends Component {
  constructor(props) {
    super(props);

    this.state = {
      explorerWidth: constants.explorerWidth
    };
    
    this.props.dispatch(refreshSpaces());
    this.props.dispatch(refreshCurrentSpace());
    this.poll = setInterval(this.handlePollSpaces, POLLING_INTERVAL);
  }

  componentWillUnmount() {
    clearInterval(this.poll);
  }

  @autobind
  handlePollSpaces() {
    this.props.dispatch(refreshSpaces());
    this.props.dispatch(refreshCurrentSpace());
  }

  render() {
    return (
      <div className="SpacesView">
        {this.props.isLoaded && 
          <>
            <Resizable
              className="ExplorerColumn"
              defaultSize={{  width: constants.explorerWidth }}
              size={{ width: this.state.explorerWidth }}
              onResizeStop={(e, direction, ref, d) => {
                this.setState({
                  explorerWidth: this.state.explorerWidth + d.width
                });
              }}
              minWidth='180'
              maxWidth='450'
              enable={{ 
                top:false, right:true, bottom:false, left:false, topRight:false, 
                bottomRight:false, bottomLeft:false, topLeft:false 
              }}
            >
              <SpaceExplorer />
            </Resizable>
            <Space />
          </>
        }

        {/* TODO: Loader. */}
        {!this.props.isLoaded && 
          <></>
        }
      </div>
    );
  }
}

SpacesView.propTypes = {
  dispatch: PropTypes.func,
  isLoaded: PropTypes.bool.isRequired
};

export default ReactRedux.connect(state => {
  return {
    isLoaded: state.spaces.isLoaded
  };
})(SpacesView);
